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
	mux.Get("/status/new", http.HandlerFunc(app.createStatusForm))
	mux.Post("/status/new", http.HandlerFunc(app.createStatus))
	mux.Del("/status/:id", http.HandlerFunc(app.deleteStatus))

	return chain.Then(mux)
}
