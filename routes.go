package main

import (
	"net/http"

	"github.com/bmizerany/pat"
	"github.com/lobre/tdispo/bow"
)

func (app *application) routes() http.Handler {
	chain := bow.DynChain.Append(app.session.Enable, app.recognizeGuest)

	mux := pat.New()

	mux.Get("/assets/", app.assets.FileServer())

	// cookie authentication
	mux.Get("/whoareyou", chain.ThenFunc(app.whoAreYou))
	mux.Post("/iam/:id", chain.ThenFunc(app.iAm))
	mux.Get("/admin", chain.ThenFunc(app.admin))
	mux.Get("/noadmin", chain.Append(app.requireAdmin).ThenFunc(app.noAdmin))

	// status
	mux.Get("/status", chain.Append(app.requireAdmin).ThenFunc(app.findStatuses))
	mux.Get("/status/new", chain.Append(app.requireAdmin).ThenFunc(app.createStatusForm))
	mux.Post("/status/new", chain.Append(app.requireAdmin).ThenFunc(app.createStatus))
	mux.Del("/status/:id", chain.Append(app.requireAdmin).ThenFunc(app.deleteStatus))

	// guests
	mux.Get("/guests", chain.Append(app.requireAdmin).ThenFunc(app.findGuests))
	mux.Get("/guests/new", chain.Append(app.requireAdmin).ThenFunc(app.createGuestForm))
	mux.Post("/guests/new", chain.Append(app.requireAdmin).ThenFunc(app.createGuest))
	mux.Get("/guests/:id/edit", chain.Append(app.requireAdmin).ThenFunc(app.updateGuestForm))
	mux.Post("/guests/:id/edit", chain.Append(app.requireAdmin).ThenFunc(app.updateGuest))
	mux.Del("/guests/:id", chain.Append(app.requireAdmin).ThenFunc(app.deleteGuest))

	// events
	mux.Get("/", chain.Append(requireRecognition).ThenFunc(app.findEvents))
	mux.Get("/new", chain.Append(app.requireAdmin).ThenFunc(app.createEventForm))
	mux.Post("/new", chain.Append(app.requireAdmin).ThenFunc(app.createEvent))
	mux.Put("/:event/participation/:guest", chain.Append(requireRecognition).ThenFunc(app.participate))
	mux.Get("/:id/edit", chain.Append(app.requireAdmin).ThenFunc(app.updateEventForm))
	mux.Post("/:id/edit", chain.Append(app.requireAdmin).ThenFunc(app.updateEvent))
	mux.Get("/:id", chain.Append(requireRecognition).ThenFunc(app.findEventByID))
	mux.Del("/:id", chain.Append(app.requireAdmin).ThenFunc(app.deleteEvent))

	return bow.StdChain.Then(mux)
}
