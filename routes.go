package main

import (
	"net/http"

	"github.com/bmizerany/pat"
	"github.com/justinas/alice"
)

func (app *application) routes() http.Handler {
	chain := alice.New(app.recoverPanic, app.logRequest, secureHeaders, app.session.Enable)

	mux := pat.New()

	// status
	mux.Get("/status", http.HandlerFunc(app.findStatuses))
	mux.Get("/status/new", http.HandlerFunc(app.createStatusForm))
	mux.Post("/status/new", http.HandlerFunc(app.createStatus))
	mux.Del("/status/:id", http.HandlerFunc(app.deleteStatus))

	// guests
	mux.Get("/guests", http.HandlerFunc(app.findGuests))
	mux.Get("/guests/new", http.HandlerFunc(app.createGuestForm))
	mux.Post("/guests/new", http.HandlerFunc(app.createGuest))
	mux.Get("/guests/:id/edit", http.HandlerFunc(app.updateGuestForm))
	mux.Post("/guests/:id/edit", http.HandlerFunc(app.updateGuest))
	mux.Del("/guests/:id", http.HandlerFunc(app.deleteGuest))

	// events
	mux.Get("/", http.HandlerFunc(app.findEvents))
	mux.Get("/new", http.HandlerFunc(app.createEventForm))
	mux.Post("/new", http.HandlerFunc(app.createEvent))
	mux.Get("/:id/edit", http.HandlerFunc(app.updateEventForm))
	mux.Post("/:id/edit", http.HandlerFunc(app.updateEvent))
	mux.Put("/:id/participation", http.HandlerFunc(app.participate))
	mux.Get("/:id", http.HandlerFunc(app.findEventByID))
	mux.Del("/:id", http.HandlerFunc(app.deleteEvent))

	return chain.Then(mux)
}
