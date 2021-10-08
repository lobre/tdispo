package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/justinas/nosurf"
)

type contextKey string

const (
	contextKeyLayout contextKey = contextKey("layout")

	defaultLayout        = "base"
	defaultLayoutPartial = "partial"
	partialPrefix        = "_"
	layoutsFolder        = "layouts"
)

type view struct {
	*template.Template
	partial bool
}

type Views struct {
	views       map[string]view
	DefaultData func(*http.Request, interface{}) interface{}
}

// The Parse function walks a filesystem from the root folder to discover and parse
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
func (v *Views) Parse(fsys fs.FS, root string, funcs template.FuncMap) error {
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

	v.views = make(map[string]view)

	for _, page := range pages {
		tmpl, err := parseTemplate(fsys, funcs, page, append(layouts, partials...))
		if err != nil {
			return err
		}

		v.views[templateName(page)] = view{Template: tmpl, partial: false}
	}

	for _, partial := range partials {
		// we also include the current partial as a duplicate but it is not a big deal.
		tmpl, err := parseTemplate(fsys, funcs, partial, append(layouts, partials...))
		if err != nil {
			return err
		}

		v.views[templateName(partial)] = view{Template: tmpl, partial: true}
	}

	return nil
}

// Render renders a given view or partial, and executes the correct layout template.
// Partials only have a configurable default layout, while the layout can be altered more
// precisely for full pages using the applyLayout middleware.
func (v *Views) Render(w http.ResponseWriter, r *http.Request, name string, data interface{}) error {
	view, ok := v.views[name]
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

	// override and use the default layout for partials
	if view.partial {
		layout = filepath.Join(layoutsFolder, defaultLayoutPartial)
	}

	// default layout not found, defaulting to main
	if view.Lookup(layout) == nil {
		layout = "main"
	}

	if v.DefaultData != nil {
		data = v.DefaultData(r, data)
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

// The WithLayout helper returns a shallow copy of the request but
// with the layout applied on the context.
func WithLayout(r *http.Request, layout string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyLayout, layout)
	return r.WithContext(ctx)
}

// The ApplyLayout middleware applies a specific layout for the rendering of the view.
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

// logRequest is a middleware that logs request to the application logger.
func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.logger.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
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

// secureHeaders is a middleware that injects headers in the response
// to prevent XSS and Clickjacking attacks.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		next.ServeHTTP(w, r)
	})
}

// injectCSRFCookie injects an encrypted CSRF token in a cookie. That same token
// is used as a hidden field in forms (from nosurf.Token()).
// On the form submission, the server checks that these two values match.
// So directly trying to post a request to our secured endpoint without this parameter would fail.
// The only way to submit the form is from our frontend.
func injectCSRFCookie(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
	})

	return csrfHandler
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

// Form will validate form data against a particular set of rules.
// If an error occurs, it will store an error message associated with
// the field.
type Form struct {
	url.Values
	errors map[string][]string
}

// New creates a new Form taking data as entry.
func NewForm(data url.Values) *Form {
	return &Form{
		data,
		map[string][]string{},
	}
}

// Error retrieves the first error message for a given
// field from the errors map.
func (f *Form) Error(field string) string {
	errors := f.errors[field]
	if len(errors) == 0 {
		return ""
	}
	return errors[0]
}

// Required checks that specific fields in the form
// data are present and not blank. If any fields fail this check,
// add the appropriate message to the form errors.
func (f *Form) Required(fields ...string) {
	for _, field := range fields {
		value := f.Get(field)
		if strings.TrimSpace(value) == "" {
			f.CustomError(field, "This field cannot be blank")
		}
	}
}

// MinLength checks that a specific field in the form contains
// a minimum number of characters. If the check fails, then add
// the appropriate message to the form errors.
func (f *Form) MinLength(field string, d int) {
	value := f.Get(field)
	if value == "" {
		return
	}
	if utf8.RuneCountInString(value) < d {
		f.CustomError(field, fmt.Sprintf("This field is too short (minimum is %d characters)", d))
	}
}

// MaxLength checks that a specific field in the form contains
// a maximum number of characters. If the check fails, then add
// the appropriate message to the form errors.
func (f *Form) MaxLength(field string, d int) {
	value := f.Get(field)
	if value == "" {
		return
	}
	// check proper characters instead of bytes
	if utf8.RuneCountInString(value) > d {
		f.CustomError(field, fmt.Sprintf("This field is too long (maximum is %d characters)", d))
	}
}

// PermittedValues checks that a specific field in the form matches
// one of a set of specific permitted values. If the check fails,
// then add the appropriate message to the form errors.
func (f *Form) PermittedValues(field string, opts ...string) {
	value := f.Get(field)
	if value == "" {
		return
	}
	for _, opt := range opts {
		if value == opt {
			return
		}
	}
	f.CustomError(field, "This field is invalid")
}

// MatchesPattern checks that a specific field in the form matches
// a regular expression. If the check fails, then add the appropriate
// message to the form errors.
func (f *Form) MatchesPattern(field string, pattern *regexp.Regexp) {
	value := f.Get(field)
	if value == "" {
		return
	}
	if !pattern.MatchString(value) {
		f.CustomError(field, "This field is invalid")
	}
}

// IsEmail checks that a specific field in the form is a correct email.
func (f *Form) IsEmail(field string) {
	value := f.Get(field)
	if _, err := mail.ParseAddress(value); err != nil {
		f.CustomError(field, "This field is not a valid email")
	}
}

// IsInteger checks that a specific field in the form is an integer.
func (f *Form) IsInteger(fields ...string) {
	for _, field := range fields {
		value := f.Get(field)
		if _, err := strconv.Atoi(value); err != nil {
			f.CustomError(field, "This field is not a valid integer")
		}
	}
}

// CustomError adds a specific error for a field.
func (f *Form) CustomError(field, msg string) {
	f.errors[field] = append(f.errors[field], msg)
}

// Valid returns true if there are no errors in the form.
func (f *Form) Valid() bool {
	return len(f.errors) == 0
}
