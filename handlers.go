package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
)

type templateData struct {
	Statuses []*Status
	Event    *Event
	Events   []*Event
	Guest    *Guest
	Guests   []*Guest
}

// The serverError helper writes an error message and stack trace to the errorLog,
// then sends a generic 500 Internal Server Error response to the user.
func serverError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	log.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// The clientError helper sends a specific status code and corresponding description
// to the user. This should be used to send responses when there's a problem with the
// request that the user sent.
func clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// render will render the response.
// Write the template to the buffer, instead of straight to the http.ResponseWriter.
// This allows to deal with runtime errors in the rendering of the template.
func (app *app) render(w http.ResponseWriter, r *http.Request, name string, td *templateData) {
	buf := new(bytes.Buffer)

	tmpl := app.ts[name]
	if tmpl == nil {
		serverError(w, errors.New("template not found"))
		return
	}

	err := tmpl.Execute(buf, td)
	if err != nil {
		serverError(w, err)
		return
	}

	buf.WriteTo(w)
}

func (app *app) home(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home", &templateData{})
}

func (app *app) findStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "find_statuses", &templateData{
		Statuses: statuses,
	})
}

func (app *app) createStatusForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "create_status_form", &templateData{})
}

func (app *app) createStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		clientError(w, http.StatusBadRequest)
		return
	}

	s := Status{
		Label: r.FormValue("label"),
	}

	err = app.statusService.CreateStatus(r.Context(), &s)
	if err != nil {
		serverError(w, err)
		return
	}

	http.Redirect(w, r, "/status", http.StatusSeeOther)
}

func (app *app) deleteStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	err = app.statusService.DeleteStatus(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}
}

func (app *app) findEvents(w http.ResponseWriter, r *http.Request) {
	events, _, err := app.eventService.FindEvents(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "find_events", &templateData{
		Events: events,
	})
}

func (app *app) findEventByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	event, err := app.eventService.FindEventByID(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "event", &templateData{
		Event: event,
	})
}

func (app *app) createEventForm(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "create_event_form", &templateData{
		Statuses: statuses,
	})
}

func (app *app) createEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		clientError(w, http.StatusBadRequest)
		return
	}

	statusID, err := strconv.Atoi(r.FormValue("status"))
	if err != nil {
		serverError(w, err)
		return
	}

	evt := Event{
		Title:    r.FormValue("title"),
		Desc:     r.FormValue("desc"),
		StatusID: statusID,
	}

	err = app.eventService.CreateEvent(r.Context(), &evt)
	if err != nil {
		serverError(w, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/events/%d", evt.ID), http.StatusSeeOther)
}

func (app *app) updateEventForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	evt, err := app.eventService.FindEventByID(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}

	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "update_event_form", &templateData{
		Event:    evt,
		Statuses: statuses,
	})
}

func (app *app) updateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	statusID, err := strconv.Atoi(r.FormValue("status"))
	if err != nil {
		serverError(w, err)
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
		serverError(w, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/events/%d", id), http.StatusSeeOther)
}

func (app *app) deleteEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	err = app.eventService.DeleteEvent(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}
}

func (app *app) findGuests(w http.ResponseWriter, r *http.Request) {
	guests, _, err := app.guestService.FindGuests(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "find_guests", &templateData{
		Guests: guests,
	})
}

func (app *app) createGuestForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "create_guest_form", &templateData{})
}

func (app *app) createGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		clientError(w, http.StatusBadRequest)
		return
	}

	guest := Guest{
		Name:  r.FormValue("name"),
		Email: r.FormValue("email"),
	}

	err = app.guestService.CreateGuest(r.Context(), &guest)
	if err != nil {
		serverError(w, err)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}

func (app *app) updateGuestForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	guest, err := app.guestService.FindGuestByID(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "update_guest_form", &templateData{
		Guest: guest,
	})
}

func (app *app) updateGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
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
		serverError(w, err)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}

func (app *app) deleteGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	err = app.guestService.DeleteGuest(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}
