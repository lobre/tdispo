package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lobre/tdispo/bow"
)

type Event struct {
	ID          int
	Title       string
	StartsAt    time.Time
	EndsAt      sql.NullTime
	Description sql.NullString

	StatusID int
	Status   *Status

	// This is only set when returning a single event.
	Participations []*Participation
}

// Upcoming returns true if the event is in the future,
// false otherwise.
func (evt *Event) Upcoming() bool {
	today := time.Now()
	return evt.StartsAt.After(today)
}

// ExtractParticipation extracts the participation of the given guest from an event.
// The participation is removed from the event itself and returned.
func (evt *Event) ExtractParticipation(guest *Guest) *Participation {
	var guestPart *Participation

	for i, part := range evt.Participations {
		if part.Guest.ID == guest.ID {
			guestPart = part
			// remove from list
			evt.Participations = append(evt.Participations[:i], evt.Participations[i+1:]...)
			break
		}
	}

	return guestPart
}

type EventFilter struct {
	ID      *int
	IDNotIn []int
	Title   *string
	Past    *bool
}

// EventUpdate represents a set of fields to be updated via UpdateEvent
type EventUpdate struct {
	Title       *string
	StartsAt    *time.Time
	EndsAt      *sql.NullTime
	Description *sql.NullString
	StatusID    *int
}

type EventService struct {
	db *bow.DB
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

	// create participations with no value for unanswered guests
	if err := attachUnansweredGuests(ctx, tx, event); err != nil {
		return nil, err
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

		// attach participations for this event
		event.Participations, _, err = findParticipationsByEvent(ctx, tx, event.ID)
		if err != nil {
			return nil, 0, err
		}

		// create participations with no value for unanswered guests
		if err := attachUnansweredGuests(ctx, tx, event); err != nil {
			return nil, 0, err
		}

		sort.Sort(ByGuestName(event.Participations))
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

	order := "ASC"
	if filter.Past != nil {
		if *filter.Past {
			where = append(where, "starts_at < date('now')")
			order = "DESC"
		} else {
			where = append(where, "starts_at >= date('now')")
		}
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT
			id,
			title,
			starts_at,
			ends_at,
			description,
			status,
			COUNT(*) OVER()
		FROM events
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY starts_at `+order,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events := make([]*Event, 0)

	for rows.Next() {
		var evt Event

		err = rows.Scan(&evt.ID, &evt.Title, &evt.StartsAt, &evt.EndsAt, &evt.Description, &evt.StatusID, &n)
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
	row := tx.QueryRowContext(ctx, `SELECT id, title, starts_at, ends_at, description, status FROM events WHERE id = ?`, id)

	var evt Event
	err := row.Scan(&evt.ID, &evt.Title, &evt.StartsAt, &evt.EndsAt, &evt.Description, &evt.StatusID)
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
		`INSERT INTO events (title, starts_at, ends_at, description, status) VALUES (?, ?, ?, ?, ?)`,
		event.Title,
		event.StartsAt,
		event.EndsAt,
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

	if upd.StartsAt != nil {
		event.StartsAt = *upd.StartsAt
	}

	if upd.EndsAt != nil {
		event.EndsAt = *upd.EndsAt
	}

	if upd.Description != nil {
		event.Description = *upd.Description
	}

	if upd.StatusID != nil {
		event.StatusID = *upd.StatusID
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE events SET title = ?, starts_at = ?, ends_at = ?, description = ?, status = ? WHERE id = ?`,
		event.Title,
		event.StartsAt,
		event.EndsAt,
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
