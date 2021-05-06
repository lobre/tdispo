package main

import (
	"strconv"
	"errors"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
)

type templateData struct {
	Statuses []*Status
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

// render will render the response.
// Write the template to the buffer, instead of straight to the http.ResponseWriter.
// This allows to deal with runtime errors in the rendering of the template.
func (app *app) render(w http.ResponseWriter, r *http.Request, name string, td *templateData) {
	buf := new(bytes.Buffer)

	tmpl := app.ts[name]
	if tmpl == nil {
		serverError(w, errors.New("template not found"))
		return
	}

	err := tmpl.Execute(buf, td)
	if err != nil {
		serverError(w, err)
		return
	}

	buf.WriteTo(w)
}

func (app *app) home(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home", &templateData{})
}

func (app *app) findStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, _, err := app.statusService.FindStatuses(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}

	app.render(w, r, "status", &templateData{
		Statuses: statuses,
	})
}

func (app *app) createStatusForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "status_new", &templateData{})
}

func (app *app) createStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		clientError(w, http.StatusBadRequest)
		return
	}

	s := Status{
		Label: r.FormValue("label"),
	}

	err = app.statusService.CreateStatus(r.Context(), &s)
	if err != nil {
		serverError(w, err)
		return
	}

	http.Redirect(w, r, "/status", http.StatusSeeOther)
}

func (app *app) deleteStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil {
		serverError(w, err)
		return
	}

	err = app.statusService.DeleteStatus(r.Context(), id)
	if err != nil {
		serverError(w, err)
		return
	}
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}
