package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
)

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home", &data{})
}

// TODO: implement
// function should set the cookie and redirect to home
func (app *application) login(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "login", &data{})
}

// TODO: implement
// function should unset the cookie and redirect to home
func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "logout", &data{})
}

func (app *application) findStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	js, err := json.Marshal(statuses)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.render(w, r, "find_statuses", &data{
		StatusesJS: template.JS(js),
	})
}

func (app *application) createStatusForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "create_status_form", &data{})
}

func (app *application) createStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	s := Status{
		Label: r.FormValue("label"),
	}

	err = app.statusService.CreateStatus(r.Context(), &s)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/status", http.StatusSeeOther)
}

func (app *application) findEvents(w http.ResponseWriter, r *http.Request) {
	events, _, err := app.eventService.FindEvents(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.render(w, r, "find_events", &data{
		Events: events,
	})
}

func (app *application) findEventByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	event, err := app.eventService.FindEventByID(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	allGuests, _, err := app.guestService.FindGuests(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.render(w, r, "event", &data{
		Event:  event,
		Guests: allGuests,
	})
}

func (app *application) createEventForm(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.render(w, r, "create_event_form", &data{
		Statuses: statuses,
	})
}

func (app *application) createEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	statusID, err := strconv.Atoi(r.FormValue("status"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	evt := Event{
		Title:    r.FormValue("title"),
		Desc:     r.FormValue("desc"),
		StatusID: statusID,
	}

	err = app.eventService.CreateEvent(r.Context(), &evt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/events/%d", evt.ID), http.StatusSeeOther)
}

func (app *application) updateEventForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	evt, err := app.eventService.FindEventByID(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.render(w, r, "update_event_form", &data{
		Event:    evt,
		Statuses: statuses,
	})
}

func (app *application) updateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	statusID, err := strconv.Atoi(r.FormValue("status"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	title := r.FormValue("title")
	desc := r.FormValue("desc")

	upd := EventUpdate{
		Title:    &title,
		Desc:     &desc,
		StatusID: &statusID,
	}

	_, err = app.eventService.UpdateEvent(r.Context(), id, upd)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/events/%d", id), http.StatusSeeOther)
}

func (app *application) findGuests(w http.ResponseWriter, r *http.Request) {
	guests, _, err := app.guestService.FindGuests(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.render(w, r, "find_guests", &data{
		Guests: guests,
	})
}

func (app *application) createGuestForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "create_guest_form", &data{})
}

func (app *application) createGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	guest := Guest{
		Name:  r.FormValue("name"),
		Email: r.FormValue("email"),
	}

	err = app.guestService.CreateGuest(r.Context(), &guest)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}

func (app *application) updateGuestForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	guest, err := app.guestService.FindGuestByID(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.render(w, r, "update_guest_form", &data{
		Guest: guest,
	})
}

func (app *application) updateGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")

	upd := GuestUpdate{
		Name:  &name,
		Email: &email,
	}

	_, err = app.guestService.UpdateGuest(r.Context(), id, upd)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}
