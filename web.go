package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"runtime/debug"
)

// templateData contains all kinds of objects
// that can be returned in a template.
type templateData struct {
	Flash   string
	Form    *Form
	Partial bool

	AssistLabels map[int]string
	Event        *Event
	Events       []*Event
	Guest        *Guest
	Guests       []*Guest
	Statuses     []*Status
}

func (app *application) parseTemplates(fsys fs.FS, base string, funcs template.FuncMap) error {
	app.pages = make(map[string]*template.Template)

	pages, err := fs.Glob(fsys, filepath.Join(base, "*.html"))
	if err != nil {
		return err
	}

	for _, page := range pages {
		key := filepath.Base(page)
		tmpl := template.New(key).Funcs(funcs)

		_, err = tmpl.ParseFS(fsys, page)
		if err != nil {
			return err
		}

		_, err = tmpl.ParseFS(fsys, filepath.Join(base, "layouts", "base.html"))
		if err != nil {
			return err
		}

		_, err = tmpl.ParseFS(fsys, filepath.Join(base, "partials", "*.html"))
		if err != nil {
			return err
		}

		app.pages[key] = tmpl
	}

	partials := template.New("partials").Funcs(funcs)

	_, err = partials.ParseFS(fsys, filepath.Join(base, "partials", "*.html"))
	if err != nil {
		return err
	}

	app.partials = partials

	return nil
}

// The addDefaultData helper will automatically inject data that are common to all pages.
func (app *application) addDefaultData(data *templateData, r *http.Request) *templateData {
	if data == nil {
		data = &templateData{}
	}
	data.Flash = app.session.PopString(r, "flash")
	return data
}

// The renderPage helper will execute the template of a full html page.
// For htmx boosted requests, it will only deliver the extracted "body" from the page.
func (app *application) renderPage(w http.ResponseWriter, r *http.Request, name string, data *templateData) {
	tmpl, ok := app.pages[name]
	if !ok {
		app.serverError(w, errors.New("template not found"))
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		name = "body"
	}

	data = app.addDefaultData(data, r)

	var buf bytes.Buffer

	err := tmpl.ExecuteTemplate(&buf, name, data)
	if err != nil {
		app.serverError(w, err)
		return
	}

	_, err = buf.WriteTo(w)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

// The renderMain helper will execute the template for a page and
// will only deliver the extracted "main" from the page.
// This is useful to generate a partial containing the whole main section of a page.
// It will also render the flash template in case a message has been pushed.
func (app *application) renderMain(w http.ResponseWriter, r *http.Request, name string, data *templateData) {
	tmpl, ok := app.pages[name]
	if !ok {
		app.serverError(w, errors.New("template not found"))
		return
	}

	data = app.addDefaultData(data, r)
	data.Partial = true

	var buf bytes.Buffer

	err := tmpl.ExecuteTemplate(&buf, "main", data)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if data.Flash != "" {
		err := tmpl.ExecuteTemplate(&buf, "flash", data)
		if err != nil {
			app.serverError(w, err)
			return
		}
	}

	_, err = buf.WriteTo(w)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

// The renderPartial helper will execute the template for a partial.
// It will also render the flash template in case a message has been pushed.
func (app *application) renderPartial(w http.ResponseWriter, r *http.Request, name string, data *templateData) {
	data = app.addDefaultData(data, r)
	data.Partial = true

	var buf bytes.Buffer

	err := app.partials.ExecuteTemplate(&buf, name, data)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if data.Flash != "" {
		err := app.partials.ExecuteTemplate(&buf, "flash", data)
		if err != nil {
			app.serverError(w, err)
			return
		}
	}

	_, err = buf.WriteTo(w)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

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
