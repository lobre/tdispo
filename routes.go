package main

import (
	"net/http"

	"github.com/bmizerany/pat"
	"github.com/justinas/alice"
)

func (app *application) routes() http.Handler {
	stdChain := alice.New(app.recoverPanic, app.logRequest, secureHeaders)
	dynChain := alice.New(app.session.Enable, injectCSRFCookie, app.recognizeGuest)

	mux := pat.New()

	mux.Get("/static/", http.FileServer(http.FS(assets)))

	// cookie authentication
	mux.Get("/whoareyou", dynChain.ThenFunc(app.whoareyou))
	mux.Post("/iam/:id", dynChain.ThenFunc(app.iam))
	mux.Get("/admin", dynChain.ThenFunc(app.admin))
	mux.Get("/noadmin", dynChain.Append(app.requireAdmin).ThenFunc(app.noadmin))

	// status
	mux.Get("/status", dynChain.Append(app.requireAdmin).ThenFunc(app.findStatuses))
	mux.Get("/status/new", dynChain.Append(app.requireAdmin).ThenFunc(app.createStatusForm))
	mux.Post("/status/new", dynChain.Append(app.requireAdmin).ThenFunc(app.createStatus))
	mux.Del("/status/:id", dynChain.Append(app.requireAdmin).ThenFunc(app.deleteStatus))

	// guests
	mux.Get("/guests", dynChain.Append(app.requireAdmin).ThenFunc(app.findGuests))
	mux.Get("/guests/new", dynChain.Append(app.requireAdmin).ThenFunc(app.createGuestForm))
	mux.Post("/guests/new", dynChain.Append(app.requireAdmin).ThenFunc(app.createGuest))
	mux.Get("/guests/:id/edit", dynChain.Append(app.requireAdmin).ThenFunc(app.updateGuestForm))
	mux.Post("/guests/:id/edit", dynChain.Append(app.requireAdmin).ThenFunc(app.updateGuest))
	mux.Del("/guests/:id", dynChain.Append(app.requireAdmin).ThenFunc(app.deleteGuest))

	// events
	mux.Get("/", dynChain.Append(requireRecognition).ThenFunc(app.findEvents))
	mux.Get("/new", dynChain.Append(app.requireAdmin).ThenFunc(app.createEventForm))
	mux.Post("/new", dynChain.Append(app.requireAdmin).ThenFunc(app.createEvent))
	mux.Get("/:id/edit", dynChain.Append(app.requireAdmin).ThenFunc(app.updateEventForm))
	mux.Post("/:id/edit", dynChain.Append(app.requireAdmin).ThenFunc(app.updateEvent))
	mux.Put("/:id/participation", dynChain.Append(requireRecognition).ThenFunc(app.participate))
	mux.Get("/:id", dynChain.Append(requireRecognition).ThenFunc(app.findEventByID))
	mux.Del("/:id", dynChain.Append(app.requireAdmin).ThenFunc(app.deleteEvent))

	return stdChain.Then(mux)
}
