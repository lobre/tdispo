package webapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/justinas/nosurf"
)

type contextKey int

const (
	contextKeyLayout contextKey = iota

	defaultLayout        = "base"
	defaultLayoutPartial = "partial"
	partialPrefix        = "_"
	layoutsFolder        = "layouts"
)

// Default logger used in app.
var Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

// HydrateFunc represents a func that can inject automatic
// data at the rendering of a view.
type HydrateFunc func(*http.Request, interface{}) interface{}

type view struct {
	*template.Template
	partial bool
}

// Core wraps convenient helpers, middlewares and
// a view engine to easily develop web applications.
// It is meant to be embedded in a parent application struct.
type Core struct {
	viewMap map[string]view
	hydrate HydrateFunc
}

// ServerError writes an error message and stack trace to the errorLog,
// then sends a generic 500 Internal Server Error response to the user.
func (core *Core) ServerError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	Logger.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// ClientError sends a specific status code and corresponding description
// to the user. This should be used to send responses when there's a problem with the
// request that the user sent.
func (core *Core) ClientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// ParseViews walks a filesystem from the root folder to discover and parse
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
func (core *Core) ParseViews(fsys fs.FS, root string, funcs template.FuncMap, hydrate HydrateFunc) error {
	core.viewMap = make(map[string]view)
	core.hydrate = hydrate

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
		return err
	}

	for _, page := range pages {
		tmpl, err := parseTemplate(fsys, funcs, page, append(layouts, partials...))
		if err != nil {
			return err
		}

		core.viewMap[templateName(page)] = view{Template: tmpl, partial: false}
	}

	for _, partial := range partials {
		// we also include the current partial as a duplicate but it is not a big deal.
		tmpl, err := parseTemplate(fsys, funcs, partial, append(layouts, partials...))
		if err != nil {
			return err
		}

		core.viewMap[templateName(partial)] = view{Template: tmpl, partial: true}
	}

	return nil
}

// Render renders a given view or partial, and executes the correct layout template.
// The layout for partials is hard-defined as "partial", while the layout can be altered more
// precisely for full pages using the applyLayout middleware.
// If no layout is found, the fallback will be to use the "main" template.
func (core *Core) Render(w http.ResponseWriter, r *http.Request, name string, data interface{}) error {
	view, ok := core.viewMap[name]
	if !ok {
		return fmt.Errorf("view %s not found", name)
	}

	layout, ok := r.Context().Value(contextKeyLayout).(string)
	if ok {
		layout = filepath.Join(layoutsFolder, layout)
		if view.Lookup(layout) == nil {
			return fmt.Errorf("layout %s not found", layout)
		}
	} else {
		layout = filepath.Join(layoutsFolder, defaultLayout)
	}

	// override and use the default layout for partials anyway
	if view.partial {
		layout = filepath.Join(layoutsFolder, defaultLayoutPartial)
	}

	// default layout not found, defaulting to main
	if view.Lookup(layout) == nil {
		layout = "main"
	}

	if core.hydrate != nil {
		data = core.hydrate(r, data)
	}

	var buf bytes.Buffer

	err := view.ExecuteTemplate(&buf, layout, data)
	if err != nil {
		return err
	}

	_, err = buf.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}

// templateName returns a template name from a path.
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

// parseTemplate creates a new template from the given path and parses the main and
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

// WithLayout returns a shallow copy of the request but
// with the layout applied on the context.
func WithLayout(r *http.Request, layout string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyLayout, layout)
	return r.WithContext(ctx)
}

// ApplyLayout applies a specific layout for the rendering of the view.
// It returns a function which has the correct signature to be used with alice, but it can
// also be used without.
//
// https://pkg.go.dev/github.com/justinas/alice#Constructor
func ApplyLayout(layout string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, WithLayout(r, layout))
		})
	}
}

// LogRequest is a middleware that logs the request to the application logger.
func (core *Core) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Logger.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}

// RecoverPanic gracefully handles any panic that happens in the current go routine.
// By default, panics don't shut the entire application (only the current go routine),
// but if one arise, the server will return an empty response. This middleware is taking
// care of recovering the panic and sending a regular 500 server error.
func (core *Core) RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// make the http.Server automatically close the current connection.
				w.Header().Set("Connection", "close")

				core.ServerError(w, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// SecureHeaders is a middleware that injects headers in the response
// to prevent XSS and Clickjacking attacks.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		next.ServeHTTP(w, r)
	})
}

// InjectCSRFCookie injects an encrypted CSRF token in a cookie. That same token
// is used as a hidden field in forms (from nosurf.Token()).
// On the form submission, the server checks that these two values match.
// So directly trying to post a request to our secured endpoint without this parameter would fail.
// The only way to submit the form is from our frontend.
func InjectCSRFCookie(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
	})

	return csrfHandler
}

// Run runs the http server and launches a goroutine
// to listen to os.Interrupt before stopping it gracefully.
func Run(srv *http.Server) error {
	shutdown := make(chan error)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop

		Logger.Println("shutting down server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdown <- srv.Shutdown(ctx)
	}()

	Logger.Printf("starting server on %s\n", srv.Addr)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdown
	if err != nil {
		return err
	}

	Logger.Println("server stopped")

	return nil
}
