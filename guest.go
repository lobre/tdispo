package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lobre/tdispo/webapp"
	"github.com/mattn/go-sqlite3"
)

type Guest struct {
	ID    int
	Name  string
	Email string

	// This is only set when returning a single guest.
	Participations []*Participation
}

type GuestFilter struct {
	ID      *int
	IDNotIn []int
}

// GuestUpdate represents a set of fields to be updated via UpdateGuest.
type GuestUpdate struct {
	Name  *string
	Email *string
}

type GuestService struct {
	db *webapp.DB
}

func (s *GuestService) FindGuestByID(ctx context.Context, id int) (*Guest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	guest, err := findGuestByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	// attach answered participations
	guest.Participations, _, err = findParticipationsByGuest(ctx, tx, guest.ID)
	if err != nil {
		return nil, err
	}

	var eventIDs []int
	for _, part := range guest.Participations {
		eventIDs = append(eventIDs, part.EventID)
	}

	// attach events to which the guest hasn’t answered yet
	pending, _, err := findEvents(ctx, tx, EventFilter{IDNotIn: eventIDs})
	if err != nil {
		return nil, err
	}

	// Add participations with assist that equals no for pending events
	for _, event := range pending {
		guest.Participations = append(guest.Participations, &Participation{
			Guest:  guest,
			Event:  event,
			Assist: 0,
		})
	}

	return guest, nil
}

func (s *GuestService) FindGuests(ctx context.Context, filter GuestFilter) ([]*Guest, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	return findGuests(ctx, tx, filter)
}

func (s *GuestService) CreateGuest(ctx context.Context, guest *Guest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = createGuest(ctx, tx, guest)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *GuestService) DeleteGuest(ctx context.Context, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	err = deleteGuest(ctx, tx, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *GuestService) UpdateGuest(ctx context.Context, id int, upd GuestUpdate) (*Guest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	guest, err := updateGuest(ctx, tx, id, upd)
	if err != nil {
		return nil, err
	}

	return guest, tx.Commit()
}

func findGuests(ctx context.Context, tx *sql.Tx, filter GuestFilter) (_ []*Guest, n int, err error) {
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

	rows, err := tx.QueryContext(ctx,
		`SELECT
			id,
			name,
			email,
			COUNT(*) OVER()
		FROM guests
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY name`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	guests := make([]*Guest, 0)

	for rows.Next() {
		var guest Guest

		err = rows.Scan(&guest.ID, &guest.Name, &guest.Email, &n)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, 0, ErrNoRecord
			}
			return nil, 0, err
		}

		guests = append(guests, &guest)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return guests, n, nil
}

func findGuestByID(ctx context.Context, tx *sql.Tx, id int) (*Guest, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, name, email FROM guests WHERE id = ?`, id)

	var guest Guest
	err := row.Scan(&guest.ID, &guest.Name, &guest.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecord
		}
		return nil, err
	}

	return &guest, nil
}

func createGuest(ctx context.Context, tx *sql.Tx, guest *Guest) error {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO guests (name, email) VALUES (?, ?)`,
		guest.Name,
		guest.Email,
	)
	if err != nil {
		var sqliteError sqlite3.Error
		if errors.As(err, &sqliteError) {
			if sqliteError.ExtendedCode == sqlite3.ErrConstraintUnique && strings.Contains(sqliteError.Error(), "guests.email") {
				return ErrDuplicateEmail
			}
		}
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	guest.ID = int(id)

	return nil
}

func updateGuest(ctx context.Context, tx *sql.Tx, id int, upd GuestUpdate) (*Guest, error) {
	guest, err := findGuestByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	if upd.Name != nil {
		guest.Name = *upd.Name
	}

	if upd.Email != nil {
		guest.Email = *upd.Email
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE guests SET name = ?, email = ? WHERE id = ?`,
		guest.Name,
		guest.Email,
		id,
	)
	if err != nil {
		return nil, err
	}

	return guest, nil
}

func deleteGuest(ctx context.Context, tx *sql.Tx, id int) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM guests WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return nil
}
