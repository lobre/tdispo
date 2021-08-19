package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Event struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Desc  string `json:"desc"`

	StatusID int     `json:"-"`
	Status   *Status `json:"status"`

	// List of participations to answered events.
	// This is only set when returning a single event.
	Answered []*Participation `json:"answered"`

	// List of pending guests that haven’t participated yet.
	// This is only set when returning a single event.
	Pending []*Guest `json:"pending"`
}

type EventFilter struct {
	ID      *int
	IDNotIn []int
}

// EventUpdate represents a set of fields to be updated via UpdateEvent
type EventUpdate struct {
	Title    *string
	Desc     *string
	StatusID *int // TODO: see if we need the full object
}

type EventService struct {
	db *DB
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
	event.Answered, _, err = findParticipationsByEvent(ctx, tx, event.ID)
	if err != nil {
		return nil, err
	}

	var guestIDs []int
	for _, part := range event.Answered {
		guestIDs = append(guestIDs, part.GuestID)
	}

	// attach guests who haven’t answered yet
	event.Pending, _, err = findGuests(ctx, tx, GuestFilter{IDNotIn: guestIDs})
	if err != nil {
		return nil, err
	}

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

	// at some point, we will want to have a date
	// and order by date desc
	rows, err := tx.QueryContext(ctx,
		`SELECT
			id,
			title,
			desc,
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

		err = rows.Scan(&evt.ID, &evt.Title, &evt.Desc, &evt.StatusID, &n)
		if err != nil {
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
	row := tx.QueryRowContext(ctx, `SELECT id, title, desc, status FROM events WHERE id = ?`, id)

	var evt Event
	if err := row.Scan(&evt.ID, &evt.Title, &evt.Desc, &evt.StatusID); err != nil {
		return nil, err
	}

	return &evt, nil
}

func createEvent(ctx context.Context, tx *sql.Tx, event *Event) error {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO events (title, desc, status) VALUES (?, ?, ?)`,
		event.Title,
		event.Desc,
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

	if upd.Desc != nil {
		event.Desc = *upd.Desc
	}

	if upd.StatusID != nil {
		event.StatusID = *upd.StatusID
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE events SET title = ?, desc = ?, status = ? WHERE id = ?`,
		event.Title,
		event.Desc,
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
