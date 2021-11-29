package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lobre/tdispo/webapp"
)

type Event struct {
	ID          int
	Title       string
	OccursAt    time.Time
	Description string

	StatusID int
	Status   *Status

	// This is only set when returning a single event.
	Participations []*Participation
}

type EventFilter struct {
	ID      *int
	IDNotIn []int
	Title   *string
}

// EventUpdate represents a set of fields to be updated via UpdateEvent
type EventUpdate struct {
	Title       *string
	OccursAt    *time.Time
	Description *string
	StatusID    *int
}

type EventService struct {
	db *webapp.DB
}

// FindEventByID retrieves an event and attaches participations and status.
func (s *EventService) FindEventByID(ctx context.Context, id int) (*Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	event, err := findEventByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	event.Status, err = findStatusByID(ctx, tx, event.StatusID)
	if err != nil {
		return nil, err
	}

	// attach participations for this event
	event.Participations, _, err = findParticipationsByEvent(ctx, tx, event.ID)
	if err != nil {
		return nil, err
	}

	var guestIDs []int
	for _, part := range event.Participations {
		guestIDs = append(guestIDs, part.GuestID)
	}

	// attach guests who haven’t answered yet
	pending, _, err := findGuests(ctx, tx, GuestFilter{IDNotIn: guestIDs})
	if err != nil {
		return nil, err
	}

	// Add participations with attend that equals no answer for pending guests
	for _, guest := range pending {
		event.Participations = append(event.Participations, &Participation{
			Guest:  guest,
			Event:  event,
			Attend: AttendNoAnswer,
		})
	}

	sort.Sort(ByGuestName(event.Participations))

	return event, nil
}

// FindEvents retrieves the list of events and attaches status for each of them.
func (s *EventService) FindEvents(ctx context.Context, filter EventFilter) ([]*Event, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	events, n, err := findEvents(ctx, tx, filter)
	if err != nil {
		return nil, 0, err
	}

	// attach status
	for _, event := range events {
		event.Status, err = findStatusByID(ctx, tx, event.StatusID)
		if err != nil {
			return nil, 0, err
		}
	}

	return events, n, nil
}

func (s *EventService) CreateEvent(ctx context.Context, event *Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = createEvent(ctx, tx, event)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *EventService) DeleteEvent(ctx context.Context, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = deleteEvent(ctx, tx, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *EventService) UpdateEvent(ctx context.Context, id int, upd EventUpdate) (*Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	event, err := updateEvent(ctx, tx, id, upd)
	if err != nil {
		return nil, err
	}

	return event, tx.Commit()
}

func (s *EventService) Participate(ctx context.Context, part *Participation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = participate(ctx, tx, part)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func findEvents(ctx context.Context, tx *sql.Tx, filter EventFilter) (_ []*Event, n int, err error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	if filter.ID != nil {
		where, args = append(where, "id = ?"), append(args, *filter.ID)
	}

	if filter.IDNotIn != nil {
		var placeholder []string
		for _, id := range filter.IDNotIn {
			placeholder = append(placeholder, "?")
			args = append(args, id)
		}
		where = append(where, fmt.Sprintf("id NOT IN (%s)", strings.Join(placeholder, ",")))
	}

	if filter.Title != nil {
		where, args = append(where, "title LIKE ?"), append(args, "%"+*filter.Title+"%")
	}

	// at some point, we will want to have a date
	// and order by date desc
	rows, err := tx.QueryContext(ctx,
		`SELECT
			id,
			title,
			occurs_at,
			description,
			status,
			COUNT(*) OVER()
		FROM events
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY title`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events := make([]*Event, 0)

	for rows.Next() {
		var evt Event

		err = rows.Scan(&evt.ID, &evt.Title, &evt.OccursAt, &evt.Description, &evt.StatusID, &n)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, 0, ErrNoRecord
			}
			return nil, 0, err
		}

		events = append(events, &evt)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return events, n, nil
}

func findEventByID(ctx context.Context, tx *sql.Tx, id int) (*Event, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, title, occurs_at, description, status FROM events WHERE id = ?`, id)

	var evt Event
	err := row.Scan(&evt.ID, &evt.Title, &evt.OccursAt, &evt.Description, &evt.StatusID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecord
		}
		return nil, err
	}

	return &evt, nil
}

func createEvent(ctx context.Context, tx *sql.Tx, event *Event) error {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO events (title, occurs_at, description, status) VALUES (?, ?, ?, ?)`,
		event.Title,
		event.OccursAt,
		event.Description,
		event.StatusID,
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	event.ID = int(id)

	return nil
}

func updateEvent(ctx context.Context, tx *sql.Tx, id int, upd EventUpdate) (*Event, error) {
	event, err := findEventByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	if upd.Title != nil {
		event.Title = *upd.Title
	}

	if upd.OccursAt != nil {
		event.OccursAt = *upd.OccursAt
	}

	if upd.Description != nil {
		event.Description = *upd.Description
	}

	if upd.StatusID != nil {
		event.StatusID = *upd.StatusID
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE events SET title = ?, occurs_at = ?, description = ?, status = ? WHERE id = ?`,
		event.Title,
		event.OccursAt,
		event.Description,
		event.StatusID,
		id,
	)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func deleteEvent(ctx context.Context, tx *sql.Tx, id int) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return nil
}
