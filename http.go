package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
)

// logRequest is a middleware that logs request to the application logger.
func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.logger.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}

// secureHeaders is a middleware that injects headers in the response
// to prevent XSS and Clickjacking attacks.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		next.ServeHTTP(w, r)
	})
}

// recoverPanic gracefully handles any panic that happens in the current go routine.
// By default, panics don't shut the entire application (only the current go routine),
// but if one arise, the server will return an empty response. This middleware is taking
// care of recovering the panic and sending a regular 500 server error.
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// make the http.Server automatically close the current connection.
				w.Header().Set("Connection", "close")

				app.serverError(w, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// The serverError helper writes an error message and stack trace to the errorLog,
// then sends a generic 500 Internal Server Error response to the user.
func (app *application) serverError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.logger.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// The clientError helper sends a specific status code and corresponding description
// to the user. This should be used to send responses when there's a problem with the
// request that the user sent.
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// The flash helper will add a global flash message.
func (app *application) flash(r *http.Request, msg string) {
	app.session.Put(r, "flash", app.translator.translate(msg))
}

// The addDefaultData helper will automatically inject data that are common to all pages.
func (app *application) addDefaultData(data *templateData, r *http.Request) *templateData {
	if data == nil {
		data = &templateData{}
	}
	data.Boost = app.config.boost
	data.Flash = app.session.PopString(r, "flash")
	return data
}

// The render helper will execute one or multiple templates found in the map of templates at the given key.
// It writes the templates to a buffer, instead of straight to the http.ResponseWriter.
// This allows to deal with runtime errors in the rendering of templates.
func (app *application) render(w http.ResponseWriter, r *http.Request, key string, names []string, data *templateData) {
	buf := new(bytes.Buffer)

	tmpl, ok := app.templates[key]
	if !ok {
		app.serverError(w, errors.New("template key not found"))
		return
	}

	data = app.addDefaultData(data, r)

	for _, name := range names {
		err := tmpl.ExecuteTemplate(buf, name, data)
		if err != nil {
			app.serverError(w, err)
			return
		}
	}

	buf.WriteTo(w)
}

// The renderPage helper will execute the template of a full html page.
// For htmx boosted requests, it will only deliver the extracted "body" from the page.
func (app *application) renderPage(w http.ResponseWriter, r *http.Request, name string, data *templateData) {
	if r.Header.Get("HX-Request") == "true" {
		app.render(w, r, name, []string{"body"}, data)
		return
	}
	app.render(w, r, name, []string{name}, data)
}

// The renderMain helper will execute the template for a page and
// will only deliver the extracted "main" from the page.
// This is useful to generate a partial containing the whole main section of a page.
// It will also render the flash template in case a message has been pushed.
func (app *application) renderMain(w http.ResponseWriter, r *http.Request, name string, data *templateData) {
	app.render(w, r, name, []string{"flash", "main"}, data)
}

// The renderPartial helper will execute the template for a partial.
// It will also render the flash template in case a message has been pushed.
func (app *application) renderPartial(w http.ResponseWriter, r *http.Request, name string, data *templateData) {
	app.render(w, r, "partials", []string{"flash", name}, data)
}
