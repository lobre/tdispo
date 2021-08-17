package main

import (
	"fmt"
	"net/http"
	"strconv"
)

func (app *application) courses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	var input struct {
		Item string `json:"item"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	items := []string{
		"Noisettes",
		"Chocolat",
		"Fraises",
	}

	items = append(items, input.Item)

	env := envelope{"courses": items}

	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) participate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	eventID, err := strconv.Atoi(r.Form.Get("event"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	guestID, err := strconv.Atoi(r.Form.Get("guest"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	env := envelope{
		"event_id": eventID,
		"guest_id": guestID,
	}

	app.logger.Println(env)

	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) deleteStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{}, nil)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	fmt.Println(id)
}

func (app *application) deleteEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowed(w, r)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	err = app.eventService.DeleteEvent(r.Context(), id)
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

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	err = app.guestService.DeleteGuest(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
}
