package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (app *application) findStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.render(w, r, "statuses/list", &templateData{
		Statuses: statuses,
	})
}

func (app *application) createStatusForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "statuses/create_form", &templateData{
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
		app.render(w, r, "statuses/create_form", &templateData{Form: form})
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
	if err != nil && errors.Is(err, ErrStatusUsed) {
		app.flash(r, "Can’t delete a status assigned to an existing event")
	} else if err != nil {
		app.serverError(w, err)
		return
	}

	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.render(w, withLayout(r, "partial"), "statuses/list", &templateData{
		Statuses: statuses,
	})
}

func (app *application) findEvents(w http.ResponseWriter, r *http.Request) {
	events, _, err := app.eventService.FindEvents(r.Context(), EventFilter{})
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.render(w, r, "events/list", &templateData{
		Events: events,
	})
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

	app.render(w, r, "events/details", &templateData{
		Event:      event,
		AssistText: AssistText,
	})
}

func (app *application) createEventForm(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.render(w, r, "events/create_form", &templateData{
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
		app.render(w, r, "events/create_form", &templateData{Form: form})
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

	app.render(w, r, "events/update_form", &templateData{
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

		app.render(w, r, "events/update_form", &templateData{
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

	app.render(w, withLayout(r, "partial"), "events/list", &templateData{
		Events: events,
	})
}

func (app *application) findGuests(w http.ResponseWriter, r *http.Request) {
	guests, _, err := app.guestService.FindGuests(r.Context(), GuestFilter{})
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.render(w, r, "guests/list", &templateData{
		Guests: guests,
	})
}

func (app *application) createGuestForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "guests/create_form", &templateData{
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
		app.render(w, r, "guests/create_form", &templateData{Form: form})
		return
	}

	guest := Guest{
		Name:  form.Get("name"),
		Email: form.Get("email"),
	}

	err = app.guestService.CreateGuest(r.Context(), &guest)
	if err != nil && errors.Is(err, ErrDuplicateEmail) {
		form.CustomError("email", "The email address already exists")
		app.render(w, r, "guests/create_form", &templateData{Form: form})
		return
	} else if err != nil {
		app.serverError(w, err)
		return
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

	app.render(w, r, "guests/update_form", &templateData{
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

		app.render(w, r, "guests/update_form", &templateData{
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

	app.render(w, withLayout(r, "partial"), "guests/list", &templateData{
		Guests: guests,
	})
}

func (app *application) participate(w http.ResponseWriter, r *http.Request) {
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
	form.Required("guest", "assist")
	form.IsInteger("guest", "assist")

	if !form.Valid() {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	guestID, _ := strconv.Atoi(form.Get("guest"))
	assist, _ := strconv.Atoi(form.Get("assist"))

	err = app.eventService.Participate(r.Context(), &Participation{
		EventID: id,
		GuestID: guestID,
		Assist:  assist,
	})
	if err != nil {
		app.serverError(w, err)
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

	app.render(w, r, "events/participations", &templateData{
		Event:      event,
		AssistText: AssistText,
	})
}
