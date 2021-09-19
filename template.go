package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"

	"github.com/russross/blackfriday"
)

// templateData contains all kinds of objects
// that can be returned in a template.
type templateData struct {
	Form         *Form
	Boost        bool
	Flash        string
	Statuses     []*Status
	Event        *Event
	Events       []*Event
	Guest        *Guest
	Guests       []*Guest
	AssistLabels map[int]string
}

func (app *application) parseTemplates() error {
	app.templates = make(map[string]*template.Template)

	funcs := template.FuncMap{
		"markdown":  markdown,
		"translate": app.translator.translate,
	}

	pages, err := fs.Glob(assets, "html/*.html")
	if err != nil {
		return err
	}

	for _, page := range pages {
		pageBase := filepath.Base(page)
		tmpl := template.New(pageBase).Funcs(funcs)

		_, err = tmpl.ParseFS(assets, page)
		if err != nil {
			return err
		}

		_, err = tmpl.ParseFS(assets, "html/layouts/base.html")
		if err != nil {
			return err
		}

		_, err = tmpl.ParseFS(assets, "html/partials/*.html")
		if err != nil {
			return err
		}

		app.templates[pageBase] = tmpl
	}

	partials := template.New("partials").Funcs(funcs)

	_, err = partials.ParseFS(assets, "html/partials/*.html")
	if err != nil {
		return err
	}

	app.templates["partials"] = partials

	return nil
}

func markdown(args ...interface{}) template.HTML {
	s := blackfriday.MarkdownCommon([]byte(fmt.Sprintf("%s", args...)))
	return template.HTML(s)
}
