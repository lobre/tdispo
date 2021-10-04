package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

type contextKey int

const (
	contextKeyLayout contextKey = iota

	defaultLayout        = "base"
	defaultLayoutPartial = "partial"
	partialPrefix        = "_"
	layoutsFolder        = "layouts"
)

// templateData contains all kinds of objects
// that can be returned in a template.
type templateData struct {
	Flash string
	Form  *Form

	AssistText map[int]string
	Event      *Event
	Events     []*Event
	Guest      *Guest
	Guests     []*Guest
	Statuses   []*Status
}

type view struct {
	*template.Template
	partial bool
}

// The templateName function returns a template name from a path.
// It removes the extension, removes the leading "_" from partials
// and trims the root directory.
func templateName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	if base[0:1] == partialPrefix {
		base = base[1:]
	}

	dirs := strings.Split(filepath.Dir(path), string(os.PathSeparator))
	dir := filepath.Join(dirs[1:]...)

	return filepath.Join(dir, base)
}

// The parseTemplate function creates a new template from the given path and parses the main and
// associated templates from the given filesystem. It also attached funcs.
func parseTemplate(fsys fs.FS, funcs template.FuncMap, path string, associated []string) (*template.Template, error) {
	tmpl := template.New("main").Funcs(funcs)

	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}

	_, err = tmpl.Parse(string(b))
	if err != nil {
		return nil, err
	}

	for _, path := range associated {
		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, err
		}

		tmpl.New(templateName(path)).Parse(string(b))
	}

	return tmpl, nil
}

// The parseViews function walks a filesystem from the root folder to discover and parse
// html files into views. Files starting with an underscore are partial views.
// Files in the layouts folder not starting with underscore are layouts. The rest of
// html files are full page views. The funcs parameter is a list of functions that is
// attached to views.
//
// Views, layouts and partials will be referred to with their path, but without the
// root folder, and without the file extension.
//
// Layouts will be referred to without the layouts folder neither.
//
// Partials files are named with a leading underscore to distinguish them from regular views,
// but will be referred to without the underscore.
func parseViews(fsys fs.FS, root string, funcs template.FuncMap) (map[string]view, error) {
	var pages, partials, layouts []string

	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}

		dirs := strings.Split(filepath.Dir(path), string(os.PathSeparator))

		switch {
		case filepath.Base(path)[0:1] == partialPrefix:
			partials = append(partials, path)
		case len(dirs) > 1 && dirs[1] == layoutsFolder:
			layouts = append(layouts, path)
		default:
			pages = append(pages, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	views := make(map[string]view)

	for _, page := range pages {
		tmpl, err := parseTemplate(fsys, funcs, page, append(layouts, partials...))
		if err != nil {
			return nil, err
		}

		views[templateName(page)] = view{Template: tmpl, partial: false}
	}

	for _, partial := range partials {
		// we also include the current partial as a duplicate but it is not a big deal.
		tmpl, err := parseTemplate(fsys, funcs, partial, append(layouts, partials...))
		if err != nil {
			return nil, err
		}

		views[templateName(partial)] = view{Template: tmpl, partial: true}
	}

	return views, nil
}

// The withLayout helper returns a shallow copy of the request but
// with the layout applied on the context.
func withLayout(r *http.Request, layout string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyLayout, layout)
	return r.WithContext(ctx)
}

// The applyLayout middleware applies a specific layout for the rendering of the view.
// It returns a function which has the correct signature to be used with alice, but it can
// also be used without.
//
// https://pkg.go.dev/github.com/justinas/alice#Constructor
func applyLayout(layout string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, withLayout(r, layout))
		})
	}
}

// The render helper renders a given view or partial, and executes the correct layout template.
// Partials only have a configurable default layout, while the layout can be altered more
// precisely for full pages using the applyLayout middleware.
func (app *application) render(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	view, ok := app.views[name]
	if !ok {
		app.serverError(w, fmt.Errorf("view %s not found", name))
		return
	}

	layout, ok := r.Context().Value(contextKeyLayout).(string)
	if ok {
		layout = filepath.Join(layoutsFolder, layout)
		if view.Lookup(layout) == nil {
			app.serverError(w, fmt.Errorf("layout %s not found", layout))
			return
		}
	} else {
		layout = filepath.Join(layoutsFolder, defaultLayout)
	}

	// override and use the default layout for partials
	if view.partial {
		layout = filepath.Join(layoutsFolder, defaultLayoutPartial)
	}

	// default layout not found, defaulting to main
	if view.Lookup(layout) == nil {
		layout = "main"
	}

	data = app.addDefaultData(data, r)

	var buf bytes.Buffer

	err := view.ExecuteTemplate(&buf, layout, data)
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

// The addDefaultData helper will automatically inject data that are common to all pages.
func (app *application) addDefaultData(data interface{}, r *http.Request) interface{} {
	td := data.(*templateData)
	if td == nil {
		td = &templateData{}
	}
	td.Flash = app.session.PopString(r, "flash")
	return td
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

// The flash helper adds a global flash message.
func (app *application) flash(r *http.Request, msg string) {
	app.session.Put(r, "flash", app.translator.translate(msg))
}

// The setBoosted middleware updates the current layout if it is an
// htmx boosted request to only return the content of the body instead
// of the full html page.
func (app *application) setBoosted(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("HX-Boosted") == "true" {
			r = withLayout(r, "boosted")
		}
		next.ServeHTTP(w, r)
	})
}
