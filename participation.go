package main

import (
	"context"
	"database/sql"
	"errors"
)

const (
	AttendNo int64 = iota
	AttendYes
	AttendIfNeeded
)

var AttendText = map[int64]string{
	AttendNo:       "no",
	AttendYes:      "yes",
	AttendIfNeeded: "if needed",
}

type Participation struct {
	GuestID int
	Guest   *Guest

	EventID int
	Event   *Event

	Attend sql.NullInt64
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
			if errors.Is(err, ErrNoRecord) {
				// guest has been removed, skip
				continue
			}
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
			if errors.Is(err, ErrNoRecord) {
				// event has been removed, skip
				continue
			}
			return nil, 0, err
		}

		participations = append(participations, &part)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return participations, n, nil
}

// attachUnansweredGuests injects a participation with no value
// on the event object for each guest who hasn’t answered.
// Existing participations should already have been added from database.
func attachUnansweredGuests(ctx context.Context, tx *sql.Tx, event *Event) error {
	var guestIDs []int
	for _, part := range event.Participations {
		guestIDs = append(guestIDs, part.GuestID)
	}

	pending, _, err := findGuests(ctx, tx, GuestFilter{IDNotIn: guestIDs})
	if err != nil {
		return err
	}

	// Add participations with attend that equals no answer for pending guests
	for _, guest := range pending {
		event.Participations = append(event.Participations, &Participation{
			Guest:  guest,
			Event:  event,
			Attend: sql.NullInt64{},
		})
	}

	return nil
}

// attachUnansweredEvents injects a participation with no value
// on the guest object for each event with no answer.
// Existing participations should already have been added from database.
func attachUnansweredEvents(ctx context.Context, tx *sql.Tx, guest *Guest) error {
	var eventIDs []int
	for _, part := range guest.Participations {
		eventIDs = append(eventIDs, part.EventID)
	}

	pending, _, err := findEvents(ctx, tx, EventFilter{IDNotIn: eventIDs})
	if err != nil {
		return err
	}

	// Add participations with attend that equals no answer for pending events
	for _, event := range pending {
		guest.Participations = append(guest.Participations, &Participation{
			Guest:  guest,
			Event:  event,
			Attend: sql.NullInt64{},
		})
	}

	return nil
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
