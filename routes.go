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

	// status
	mux.Get("/status", http.HandlerFunc(app.findStatuses))
	mux.Get("/status/create", http.HandlerFunc(app.createStatusForm))
	mux.Post("/status/create", http.HandlerFunc(app.createStatus))
	mux.Get("/status/:id", http.HandlerFunc(app.findStatusByID))
	mux.Del("/status/:id", http.HandlerFunc(app.deleteStatus))

	return chain.Then(mux)
}
