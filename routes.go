package main

import (
	"fmt"
	"net/http"

	"github.com/bmizerany/pat"
	"github.com/justinas/alice"
)

const api = "/api"

func (app *application) routes() http.Handler {
	chain := alice.New(app.logRequest)

	mux := pat.New()
	mux.NotFound = http.HandlerFunc(app.notFound)

	// api
	mux.Post(fmt.Sprintf("%s/participate", api), http.HandlerFunc(app.participate))
	mux.Post(fmt.Sprintf("%s/deleteStatus", api), http.HandlerFunc(app.deleteStatus))
	mux.Post(fmt.Sprintf("%s/deleteEvent", api), http.HandlerFunc(app.deleteEvent))
	mux.Post(fmt.Sprintf("%s/deleteGuest", api), http.HandlerFunc(app.deleteGuest))

	// status
	mux.Get("/status", http.HandlerFunc(app.findStatuses))
	mux.Get("/status/new", http.HandlerFunc(app.createStatusForm))
	mux.Post("/status/new", http.HandlerFunc(app.createStatus))

	// guests
	mux.Get("/guests", http.HandlerFunc(app.findGuests))
	mux.Get("/guests/new", http.HandlerFunc(app.createGuestForm))
	mux.Post("/guests/new", http.HandlerFunc(app.createGuest))
	mux.Get("/guests/:id/edit", http.HandlerFunc(app.updateGuestForm))
	mux.Post("/guests/:id/edit", http.HandlerFunc(app.updateGuest))

	// authentication
	mux.Get("/login", http.HandlerFunc(app.login))
	mux.Get("/logout", http.HandlerFunc(app.logout))

	// events
	mux.Get("/", http.HandlerFunc(app.findEvents))
	mux.Get("/new", http.HandlerFunc(app.createEventForm))
	mux.Post("/new", http.HandlerFunc(app.createEvent))
	mux.Get("/:id/edit", http.HandlerFunc(app.updateEventForm))
	mux.Post("/:id/edit", http.HandlerFunc(app.updateEvent))
	mux.Get("/:id", http.HandlerFunc(app.findEventByID))

	return chain.Then(mux)
}
