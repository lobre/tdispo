package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/lobre/tdispo/bow"
)

func (app *application) findStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		bow.ServerError(w, err)
		return
	}

	app.views.Render(w, r, "statuses/list", map[string]interface{}{
		"Statuses": statuses,
	})
}

func (app *application) createStatusForm(w http.ResponseWriter, r *http.Request) {
	app.views.Render(w, r, "statuses/create_form", map[string]interface{}{
		"Form": bow.NewForm(nil),
	})
}

func (app *application) createStatus(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	form := bow.NewForm(r.PostForm)
	form.Required("label", "color")

	if !form.Valid() {
		w.WriteHeader(http.StatusUnprocessableEntity)
		app.views.Render(w, r, "statuses/create_form", map[string]interface{}{
			"Form": form,
		})
		return
	}

	s := Status{
		Label: form.Get("label"),
		Color: form.Get("color"),
	}

	err = app.statusService.CreateStatus(r.Context(), &s)
	if err != nil {
		bow.ServerError(w, err)
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
		w.WriteHeader(http.StatusConflict)
		app.session.Put(r, "flash", "Can’t delete a status assigned to an existing event")
	} else if err != nil {
		bow.ServerError(w, err)
		return
	}

	http.Redirect(w, r, "/status", http.StatusSeeOther)
}

func (app *application) findEvents(w http.ResponseWriter, r *http.Request) {
	var filter EventFilter

	q := r.URL.Query().Get("q")
	if q != "" {
		filter.Title = &q
	}

	filter.Past = new(bool)
	past := r.URL.Query().Get("past")
	if past == "on" {
		*filter.Past = true
	}

	events, _, err := app.eventService.FindEvents(r.Context(), filter)
	if err != nil {
		bow.ServerError(w, err)
		return
	}

	app.views.Render(w, r, "events/list", map[string]interface{}{
		"Form": bow.NewForm(url.Values{
			"q":    []string{q},
			"past": []string{past},
		}),
		"Events": events,
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
			bow.ServerError(w, err)
			return
		}
	}

	var currentParticipation *Participation
	// extract participation from current guest to be able to display it first
	currentParticipation, event.Participations = extractParticipation(currentGuest(r), event.Participations)

	app.views.Render(w, r, "events/details", map[string]interface{}{
		"Event":                event,
		"CurrentParticipation": currentParticipation,
		"AttendText":           AttendText,
	})
}

func (app *application) createEventForm(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		bow.ServerError(w, err)
		return
	}

	app.views.Render(w, r, "events/create_form", map[string]interface{}{
		"Form":     bow.NewForm(nil),
		"Statuses": statuses,
	})
}

func (app *application) createEvent(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	form := bow.NewForm(r.PostForm)
	form.Required("title", "status", "startdate", "starttime")
	form.IsDate("startdate", "enddate")
	form.IsTime("starttime", "endtime")

	if form.Get("enddate") != "" && form.Get("endtime") == "" {
		form.CustomError("endtime", "This field cannot be blank as end date is filled")
	}

	if form.Get("enddate") == "" && form.Get("endtime") != "" {
		form.CustomError("enddate", "This field cannot be blank as end time is filled")
	}

	if !form.Valid() {
		statuses, _, err := app.statusService.FindStatuses(r.Context())
		if err != nil {
			bow.ServerError(w, err)
			return
		}

		w.WriteHeader(http.StatusUnprocessableEntity)
		app.views.Render(w, r, "events/create_form", map[string]interface{}{
			"Form":     form,
			"Statuses": statuses,
		})
		return
	}

	statusID, err := strconv.Atoi(form.Get("status"))
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse(layoutDatetime, fmt.Sprintf("%s %s", form.Get("startdate"), form.Get("starttime")))
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	var endDate sql.NullTime
	if form.Get("enddate") != "" || form.Get("endtime") != "" {
		endDate.Time, err = time.Parse(layoutDatetime, fmt.Sprintf("%s %s", form.Get("enddate"), form.Get("endtime")))
		if err != nil {
			bow.ClientError(w, http.StatusBadRequest)
			return
		}
		endDate.Valid = true
	}

	var description sql.NullString
	if form.Get("description") != "" {
		description.String = form.Get("description")
		description.Valid = true
	}

	evt := Event{
		Title:       form.Get("title"),
		StartsAt:    startDate,
		EndsAt:      endDate,
		Description: description,
		StatusID:    statusID,
	}

	err = app.eventService.CreateEvent(r.Context(), &evt)
	if err != nil {
		bow.ServerError(w, err)
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
		bow.ServerError(w, err)
		return
	}

	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		bow.ServerError(w, err)
		return
	}

	var endDate, endTime string
	if evt.EndsAt.Valid {
		endDate = evt.EndsAt.Time.Format(layoutDate)
		endTime = evt.EndsAt.Time.Format(layoutTime)
	}

	app.views.Render(w, r, "events/update_form", map[string]interface{}{
		"Form": bow.NewForm(url.Values{
			"title":       []string{evt.Title},
			"startdate":   []string{evt.StartsAt.Format(layoutDate)},
			"starttime":   []string{evt.StartsAt.Format(layoutTime)},
			"enddate":     []string{endDate},
			"endtime":     []string{endTime},
			"description": []string{evt.Description.String},
		}),
		"Event":    evt,
		"Statuses": statuses,
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
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	form := bow.NewForm(r.PostForm)
	form.Required("title", "status", "startdate", "starttime")
	form.IsDate("startdate", "enddate")
	form.IsTime("starttime", "endtime")

	if form.Get("enddate") != "" && form.Get("endtime") == "" {
		form.CustomError("endtime", "This field cannot be blank as end date is filled")
	}

	if form.Get("enddate") == "" && form.Get("endtime") != "" {
		form.CustomError("enddate", "This field cannot be blank as end time is filled")
	}

	if !form.Valid() {
		evt, err := app.eventService.FindEventByID(r.Context(), id)
		if err != nil {
			bow.ServerError(w, err)
			return
		}

		statuses, _, err := app.statusService.FindStatuses(r.Context())
		if err != nil {
			bow.ServerError(w, err)
			return
		}

		w.WriteHeader(http.StatusUnprocessableEntity)
		app.views.Render(w, r, "events/update_form", map[string]interface{}{
			"Form":     form,
			"Event":    evt,
			"Statuses": statuses,
		})

		return
	}

	statusID, err := strconv.Atoi(form.Get("status"))
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse(layoutDatetime, fmt.Sprintf("%s %s", form.Get("startdate"), form.Get("starttime")))
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	var endDate sql.NullTime
	if form.Get("enddate") != "" || form.Get("endtime") != "" {
		endDate.Time, err = time.Parse(layoutDatetime, fmt.Sprintf("%s %s", form.Get("enddate"), form.Get("endtime")))
		if err != nil {
			bow.ClientError(w, http.StatusBadRequest)
			return
		}
		endDate.Valid = true
	}

	title := form.Get("title")

	var description sql.NullString
	if form.Get("description") != "" {
		description.String = form.Get("description")
		description.Valid = true
	}

	upd := EventUpdate{
		Title:       &title,
		StartsAt:    &startDate,
		EndsAt:      &endDate,
		Description: &description,
		StatusID:    &statusID,
	}

	_, err = app.eventService.UpdateEvent(r.Context(), id, upd)
	if err != nil {
		bow.ServerError(w, err)
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
		bow.ServerError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) findGuests(w http.ResponseWriter, r *http.Request) {
	guests, _, err := app.guestService.FindGuests(r.Context(), GuestFilter{})
	if err != nil {
		bow.ServerError(w, err)
		return
	}

	app.views.Render(w, r, "guests/list", map[string]interface{}{
		"Guests": guests,
	})
}

func (app *application) createGuestForm(w http.ResponseWriter, r *http.Request) {
	app.views.Render(w, r, "guests/create_form", map[string]interface{}{
		"Form": bow.NewForm(nil),
	})
}

func (app *application) createGuest(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	form := bow.NewForm(r.PostForm)
	form.Required("name", "email")

	if !form.Valid() {
		w.WriteHeader(http.StatusUnprocessableEntity)
		app.views.Render(w, r, "guests/create_form", map[string]interface{}{
			"Form": form,
		})
		return
	}

	guest := Guest{
		Name:  form.Get("name"),
		Email: form.Get("email"),
	}

	err = app.guestService.CreateGuest(r.Context(), &guest)
	if err != nil && errors.Is(err, ErrDuplicateEmail) {
		form.CustomError("email", "The email address already exists")

		w.WriteHeader(http.StatusConflict)
		app.views.Render(w, r, "guests/create_form", map[string]interface{}{
			"Form": form,
		})

		return
	} else if err != nil {
		bow.ServerError(w, err)
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
			bow.ServerError(w, err)
			return
		}
	}

	app.views.Render(w, r, "guests/update_form", map[string]interface{}{
		"Form": bow.NewForm(url.Values{
			"name":  []string{guest.Name},
			"email": []string{guest.Email},
		}),
		"Guest": guest,
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
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	form := bow.NewForm(r.PostForm)
	form.Required("name", "email")

	if !form.Valid() {
		guest, err := app.guestService.FindGuestByID(r.Context(), id)
		if err != nil {
			bow.ServerError(w, err)
			return
		}

		w.WriteHeader(http.StatusUnprocessableEntity)
		app.views.Render(w, r, "guests/update_form", map[string]interface{}{
			"Form":  form,
			"Guest": guest,
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
		bow.ServerError(w, err)
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
		bow.ServerError(w, err)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}

func (app *application) participate(w http.ResponseWriter, r *http.Request) {
	eventID, err := strconv.Atoi(r.URL.Query().Get(":event"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	guestID, err := strconv.Atoi(r.URL.Query().Get(":guest"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if currentGuest(r).ID != guestID && !app.isAdmin(r) {
		bow.ClientError(w, http.StatusForbidden)
		return
	}

	err = r.ParseForm()
	if err != nil {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	form := bow.NewForm(r.PostForm)
	form.Required("attend")
	form.IsInteger("attend")

	if !form.Valid() {
		bow.ClientError(w, http.StatusBadRequest)
		return
	}

	attend, _ := strconv.Atoi(form.Get("attend"))

	err = app.eventService.Participate(r.Context(), &Participation{
		EventID: eventID,
		GuestID: guestID,
		Attend:  attend,
	})
	if err != nil {
		bow.ServerError(w, err)
		return
	}

	event, err := app.eventService.FindEventByID(r.Context(), eventID)
	if err != nil {
		if errors.Is(err, ErrNoRecord) {
			http.NotFound(w, r)
			return
		} else {
			bow.ServerError(w, err)
			return
		}
	}

	var currentParticipation *Participation
	// extract participation from current guest to be able to display it first
	currentParticipation, event.Participations = extractParticipation(currentGuest(r), event.Participations)

	app.views.Render(w, r, "events/details", map[string]interface{}{
		"Event":                event,
		"CurrentParticipation": currentParticipation,
		"AttendText":           AttendText,
	})
}

func (app *application) whoAreYou(w http.ResponseWriter, r *http.Request) {
	guests, _, err := app.guestService.FindGuests(r.Context(), GuestFilter{})
	if err != nil {
		bow.ServerError(w, err)
		return
	}

	app.views.Render(w, r, "guests/whoareyou", map[string]interface{}{
		"Guests": guests,
	})
}

func (app *application) iAm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	app.session.Put(r, "guest", id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) admin(w http.ResponseWriter, r *http.Request) {
	app.session.Put(r, "isAdmin", true)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) noAdmin(w http.ResponseWriter, r *http.Request) {
	app.session.Remove(r, "isAdmin")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
