package main

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
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

	buf := new(bytes.Buffer)

	err := tmpl.ExecuteTemplate(buf, name, data)
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

	buf := new(bytes.Buffer)

	err := tmpl.ExecuteTemplate(buf, "main", data)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if data.Flash != "" {
		err := tmpl.ExecuteTemplate(buf, "flash", data)
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

	buf := new(bytes.Buffer)

	err := app.partials.ExecuteTemplate(buf, name, data)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if data.Flash != "" {
		err := app.partials.ExecuteTemplate(buf, "flash", data)
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
