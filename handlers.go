package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	app.renderPage(w, r, "home.html", &templateData{})
}

// TODO: implement
// function should set the cookie and redirect to home
func (app *application) login(w http.ResponseWriter, r *http.Request) {
	app.renderPage(w, r, "login.html", &templateData{})
}

// TODO: implement
// function should unset the cookie and redirect to home
func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	app.renderPage(w, r, "logout.html", &templateData{})
}

func (app *application) findStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderPage(w, r, "statuses_list.html", &templateData{
		Statuses: statuses,
	})
}

func (app *application) createStatusForm(w http.ResponseWriter, r *http.Request) {
	app.renderPage(w, r, "create_status_form.html", &templateData{
		Form: NewForm(nil),
	})
}

func (app *application) createStatus(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := NewForm(r.PostForm)
	form.Required("label")

	if !form.Valid() {
		app.renderPage(w, r, "create_status_form.html", &templateData{Form: form})
		return
	}

	s := Status{
		Label: form.Get("label"),
	}

	err = app.statusService.CreateStatus(r.Context(), &s)
	if err != nil {
		app.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/status", http.StatusSeeOther)
}

func (app *application) deleteStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = app.statusService.DeleteStatus(r.Context(), id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderMain(w, r, "statuses_list.html", &templateData{
		Statuses: statuses,
	})
}

func (app *application) findEvents(w http.ResponseWriter, r *http.Request) {
	events, _, err := app.eventService.FindEvents(r.Context(), EventFilter{})
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderPage(w, r, "events_list.html", &templateData{
		Events: events,
	})
}

func (app *application) removeParticipation(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "salut")
}

func (app *application) findEventByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	event, err := app.eventService.FindEventByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNoRecord) {
			http.NotFound(w, r)
			return
		} else {
			app.serverError(w, err)
			return
		}
	}

	app.renderPage(w, r, "event_details.html", &templateData{
		Event:        event,
		AssistLabels: AssistLabels,
	})
}

func (app *application) createEventForm(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderPage(w, r, "create_event_form.html", &templateData{
		Form:     NewForm(nil),
		Statuses: statuses,
	})
}

func (app *application) createEvent(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := NewForm(r.PostForm)
	form.Required("title")

	if !form.Valid() {
		app.renderPage(w, r, "create_event_form.html", &templateData{Form: form})
		return
	}

	statusID, err := strconv.Atoi(form.Get("status"))
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	evt := Event{
		Title:    form.Get("title"),
		Desc:     form.Get("desc"),
		StatusID: statusID,
	}

	err = app.eventService.CreateEvent(r.Context(), &evt)
	if err != nil {
		app.serverError(w, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/%d", evt.ID), http.StatusSeeOther)
}

func (app *application) updateEventForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	evt, err := app.eventService.FindEventByID(r.Context(), id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderPage(w, r, "update_event_form.html", &templateData{
		Form: NewForm(url.Values{
			"title": []string{evt.Title},
			"desc":  []string{evt.Desc},
		}),
		Event:    evt,
		Statuses: statuses,
	})
}

func (app *application) updateEvent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := NewForm(r.PostForm)
	form.Required("title")

	if !form.Valid() {
		evt, err := app.eventService.FindEventByID(r.Context(), id)
		if err != nil {
			app.serverError(w, err)
			return
		}

		statuses, _, err := app.statusService.FindStatuses(r.Context())
		if err != nil {
			app.serverError(w, err)
			return
		}

		app.renderPage(w, r, "update_event_form.html", &templateData{
			Form:     form,
			Event:    evt,
			Statuses: statuses,
		})

		return
	}

	statusID, err := strconv.Atoi(form.Get("status"))
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	title := form.Get("title")
	desc := form.Get("desc")

	upd := EventUpdate{
		Title:    &title,
		Desc:     &desc,
		StatusID: &statusID,
	}

	_, err = app.eventService.UpdateEvent(r.Context(), id, upd)
	if err != nil {
		app.serverError(w, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/%d", id), http.StatusSeeOther)
}

func (app *application) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = app.eventService.DeleteEvent(r.Context(), id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	events, _, err := app.eventService.FindEvents(r.Context(), EventFilter{})
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderPage(w, r, "guests_list.html", &templateData{
		Events: events,
	})
}

func (app *application) findGuests(w http.ResponseWriter, r *http.Request) {
	guests, _, err := app.guestService.FindGuests(r.Context(), GuestFilter{})
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderPage(w, r, "guests_list.html", &templateData{
		Guests: guests,
	})
}

func (app *application) createGuestForm(w http.ResponseWriter, r *http.Request) {
	app.renderPage(w, r, "create_guest_form.html", &templateData{
		Form: NewForm(nil),
	})
}

func (app *application) createGuest(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := NewForm(r.PostForm)
	form.Required("name", "email")

	if !form.Valid() {
		app.renderPage(w, r, "create_guest_form.html", &templateData{Form: form})
		return
	}

	guest := Guest{
		Name:  form.Get("name"),
		Email: form.Get("email"),
	}

	err = app.guestService.CreateGuest(r.Context(), &guest)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			form.CustomError("email", "The email address already exists")
			app.renderPage(w, r, "create_guest_form.html", &templateData{Form: form})
			return
		} else {
			app.serverError(w, err)
			return
		}
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}

func (app *application) updateGuestForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	guest, err := app.guestService.FindGuestByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNoRecord) {
			http.NotFound(w, r)
			return
		} else {
			app.serverError(w, err)
			return
		}
	}

	app.renderPage(w, r, "update_guest_form.html", &templateData{
		Form: NewForm(url.Values{
			"name":  []string{guest.Name},
			"email": []string{guest.Email},
		}),
		Guest: guest,
	})
}

func (app *application) updateGuest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := NewForm(r.PostForm)
	form.Required("name", "email")

	if !form.Valid() {
		guest, err := app.guestService.FindGuestByID(r.Context(), id)
		if err != nil {
			app.serverError(w, err)
			return
		}

		app.renderPage(w, r, "update_guest_form.html", &templateData{
			Form:  form,
			Guest: guest,
		})

		return
	}

	name := form.Get("name")
	email := form.Get("email")

	upd := GuestUpdate{
		Name:  &name,
		Email: &email,
	}

	_, err = app.guestService.UpdateGuest(r.Context(), id, upd)
	if err != nil {
		app.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}

func (app *application) deleteGuest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = app.guestService.DeleteGuest(r.Context(), id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	guests, _, err := app.guestService.FindGuests(r.Context(), GuestFilter{})
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.renderMain(w, r, "guests_list.html", &templateData{
		Guests: guests,
	})
}
