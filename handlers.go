package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
)

type templateData struct {
}

// The serverError helper writes an error message and stack trace to the errorLog,
// then sends a generic 500 Internal Server Error response to the user.
func serverError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	log.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// The clientError helper sends a specific status code and corresponding description
// to the user. This should be used to send responses when there's a problem with the
// request that the user sent.
func clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func (app *app) render(w http.ResponseWriter, r *http.Request, name string, td *templateData) {
	buf := new(bytes.Buffer)

	// Write the template to the buffer, instead of straight to the
	// http.ResponseWriter. This allows to deal with runtime errors in the
	// rendering of the template.
	err := app.tmpl.ExecuteTemplate(buf, name, td)
	if err != nil {
		serverError(w, err)
		return
	}

	// Template has been rendered without any error, we can write it as the response.
	buf.WriteTo(w)
}

func (app *app) home(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home.html", &templateData{})
}

func (app *app) findStatuses(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "to be implemented\n") // TODO: implement
}

func (app *app) findStatusByID(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "to be implemented\n") // TODO: implement
}

func (app *app) createStatusForm(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "to be implemented\n") // TODO: implement
}

func (app *app) createStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "to be implemented\n") // TODO: implement
}

func (app *app) deleteStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "to be implemented\n") // TODO: implement
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}
