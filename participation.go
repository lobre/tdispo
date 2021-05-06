package main

import (
	"context"
	"database/sql"
)

type Participation struct {
	GuestID int
	Guest   *Guest

	EventID int
	Event   *Event

	Assist int
}

const (
	AssistNo = iota
	AssistYes
	AssistMaybe
)

// findParticipationsByEvent fetches the participations related to a specific event.
// For each participation, the guest is attached.
func findParticipationsByEvent(ctx context.Context, tx *sql.Tx, id int) (_ []*Participation, n int, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT
			guest_id,
			event_id,
			assist,
			COUNT(*) OVER()
		FROM participations
		WHERE event_id = ?`,
		id,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	participations := make([]*Participation, 0)

	for rows.Next() {
		var part Participation

		err = rows.Scan(&part.GuestID, &part.EventID, &part.Assist, &n)
		if err != nil {
			return nil, 0, err
		}

		// attach guest
		part.Guest, err = findGuestByID(ctx, tx, part.GuestID)
		if err != nil {
			return nil, 0, err
		}

		participations = append(participations, &part)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return participations, n, nil
}

// findParticipationsByGuest fetches the participations related to a specific guest.
// For each participation, the event is attached.
func findParticipationsByGuest(ctx context.Context, tx *sql.Tx, id int) (_ []*Participation, n int, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT
			guest_id,
			event_id,
			assist,
			COUNT(*) OVER()
		FROM participations
		WHERE guest_id = ?`,
		id,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	participations := make([]*Participation, 0)

	for rows.Next() {
		var part Participation

		err = rows.Scan(&part.GuestID, &part.EventID, &part.Assist, &n)
		if err != nil {
			return nil, 0, err
		}

		// attach event
		part.Event, err = findEventByID(ctx, tx, part.EventID)
		if err != nil {
			return nil, 0, err
		}

		participations = append(participations, &part)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return participations, n, nil
}
