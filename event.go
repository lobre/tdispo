package main

import (
	"context"
	"database/sql"
)

type Event struct {
	ID    int
	Title string
	Desc  string

	StatusID int
	Status   *Status

	// List of associated participations.
	// This is only set when returning a single event.
	Participations []*Participation
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

func NewEventService(db *DB) *EventService {
	return &EventService{db: db}
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

	event.Participations, _, err = findParticipationsByEvent(ctx, tx, event.ID)
	if err != nil {
		return nil, err
	}

	return event, nil
}

// FindEvents retrieves the list of events and attaches status for each of them.
func (s *EventService) FindEvents(ctx context.Context) ([]*Event, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	events, n, err := findEvents(ctx, tx)
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

func findEvents(ctx context.Context, tx *sql.Tx) (_ []*Event, n int, err error) {
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
		ORDER BY title`,
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
		event.Status,
		id,
	)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func deleteEvent(ctx context.Context, tx *sql.Tx, id int) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return nil
}
