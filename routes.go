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

	mux.Get("/", http.HandlerFunc(app.home))

	// authentication
	mux.Get("/login", http.HandlerFunc(app.login))
	mux.Get("/logout", http.HandlerFunc(app.logout))

	// events
	// TODO: make events at root /
	mux.Get("/events", http.HandlerFunc(app.findEvents))
	mux.Get("/events/new", http.HandlerFunc(app.createEventForm))
	mux.Post("/events/new", http.HandlerFunc(app.createEvent))
	mux.Get("/events/:id/edit", http.HandlerFunc(app.updateEventForm))
	mux.Post("/events/:id/edit", http.HandlerFunc(app.updateEvent))
	mux.Get("/events/:id", http.HandlerFunc(app.findEventByID))

	// guests
	mux.Get("/guests", http.HandlerFunc(app.findGuests))
	mux.Get("/guests/new", http.HandlerFunc(app.createGuestForm))
	mux.Post("/guests/new", http.HandlerFunc(app.createGuest))
	mux.Get("/guests/:id/edit", http.HandlerFunc(app.updateGuestForm))
	mux.Post("/guests/:id/edit", http.HandlerFunc(app.updateGuest))

	// status
	mux.Get("/status", http.HandlerFunc(app.findStatuses))
	mux.Get("/status/new", http.HandlerFunc(app.createStatusForm))
	mux.Post("/status/new", http.HandlerFunc(app.createStatus))

	// api
	mux.Post(fmt.Sprintf("%s/participate", api), http.HandlerFunc(app.participate))
	mux.Post(fmt.Sprintf("%s/deleteStatus", api), http.HandlerFunc(app.deleteStatus))
	mux.Post(fmt.Sprintf("%s/deleteEvent", api), http.HandlerFunc(app.deleteEvent))
	mux.Post(fmt.Sprintf("%s/deleteGuest", api), http.HandlerFunc(app.deleteGuest))

	return chain.Then(mux)
}
