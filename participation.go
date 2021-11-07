package main

import (
	"context"
	"database/sql"
)

const (
	AttendNoAnswer int = iota
	AttendNo
	AttendYes
	AttendIfNeeded
)

var AttendText = map[int]string{
	AttendNoAnswer: "no answer",
	AttendNo:       "no",
	AttendYes:      "yes",
	AttendIfNeeded: "if needed",
}

type Participation struct {
	GuestID int
	Guest   *Guest

	EventID int
	Event   *Event

	Attend int
}

// findParticipationsByEvent fetches the participations related to a specific event.
// For each participation, the guest is attached.
func findParticipationsByEvent(ctx context.Context, tx *sql.Tx, id int) (_ []*Participation, n int, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT
			guest_id,
			event_id,
			attend,
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

		err = rows.Scan(&part.GuestID, &part.EventID, &part.Attend, &n)
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
			attend,
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

		err = rows.Scan(&part.GuestID, &part.EventID, &part.Attend, &n)
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

func participate(ctx context.Context, tx *sql.Tx, part *Participation) error {
	_, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO participations (guest_id, event_id, attend) VALUES (?, ?, ?)`,
		part.GuestID,
		part.EventID,
		part.Attend,
	)
	if err != nil {
		return err
	}

	return nil
}

// ByGuestName implements sort.Interface based on the Name field of the Guest.
type ByGuestName []*Participation

func (parts ByGuestName) Len() int           { return len(parts) }
func (parts ByGuestName) Less(i, j int) bool { return parts[i].Guest.Name < parts[j].Guest.Name }
func (parts ByGuestName) Swap(i, j int)      { parts[i], parts[j] = parts[j], parts[i] }

// extractParticipation extracts the participation of the given guest and returns the list of
// participations without it.
func extractParticipation(guest *Guest, parts []*Participation) (*Participation, []*Participation) {
	var current *Participation
	for i, part := range parts {
		if part.Guest.ID == guest.ID {
			current = part
			// remove from list
			parts = append(parts[:i], parts[i+1:]...)
			break
		}
	}
	return current, parts
}
