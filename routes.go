package main

import (
	"github.com/bmizerany/pat"
	"github.com/justinas/alice"
	"net/http"
)

func (app *app) routes() http.Handler {
	chain := alice.New(logRequest)

	mux := pat.New()
	mux.Get("/", http.HandlerFunc(app.home))

	// events
	mux.Get("/events", http.HandlerFunc(app.findEvents))
	mux.Get("/events/new", http.HandlerFunc(app.createEventForm))
	mux.Post("/events/new", http.HandlerFunc(app.createEvent))
	mux.Get("/events/:id/edit", http.HandlerFunc(app.updateEventForm))
	mux.Post("/events/:id/edit", http.HandlerFunc(app.updateEvent))
	mux.Get("/events/:id", http.HandlerFunc(app.findEventByID))
	mux.Del("/events/:id", http.HandlerFunc(app.deleteEvent))

	// status
	mux.Get("/status", http.HandlerFunc(app.findStatuses))
	mux.Get("/status/new", http.HandlerFunc(app.createStatusForm))
	mux.Post("/status/new", http.HandlerFunc(app.createStatus))
	mux.Del("/status/:id", http.HandlerFunc(app.deleteStatus))

	return chain.Then(mux)
}
