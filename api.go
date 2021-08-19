package main

import (
	"errors"
	"net/http"
)

func (app *application) deleteStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	err := app.readJSON(w, r, &req)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	if req.ID == 0 {
		app.badRequest(w, r, errors.New("Missing id field in request."))
		return
	}

	err = app.statusService.DeleteStatus(r.Context(), req.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{}, nil)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
}

func (app *application) deleteGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	err := app.readJSON(w, r, &req)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	if req.ID == 0 {
		app.badRequest(w, r, errors.New("Missing id field in request."))
		return
	}

	err = app.guestService.DeleteGuest(r.Context(), req.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{}, nil)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
}

func (app *application) participate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	var req struct {
		GuestID int `json:"guest_id"`
		EventID int `json:"event_id"`
		Assist  int `json:"assist"`
	}

	err := app.readJSON(w, r, &req)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	if req.GuestID == 0 {
		app.badRequest(w, r, errors.New("Missing guest_id field in request"))
		return
	}

	if req.EventID == 0 {
		app.badRequest(w, r, errors.New("Missing event_id field in request"))
		return
	}

	part := Participation{
		GuestID: req.GuestID,
		EventID: req.EventID,
		Assist:  req.Assist,
	}

	err = app.eventService.Participate(r.Context(), &part)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{}, nil)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
}

func (app *application) deleteEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	// to implement
}
