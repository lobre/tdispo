package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/russross/blackfriday"
)

//go:embed html/*.html migrations/*.sql translations/*.csv
var assets embed.FS

type config struct {
	port int
	dsn  string
	lang string
}

type application struct {
	config config
	logger *log.Logger

	templates  map[string]*template.Template
	translator *translator

	statusService *StatusService
	guestService  *GuestService
	eventService  *EventService
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 8080, "http server port")
	flag.StringVar(&cfg.dsn, "dsn", "tdispo.db", "database datasource name")
	flag.StringVar(&cfg.lang, "lang", "en", "language of the application")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	db := NewDB(cfg.dsn)
	if err := db.Open(); err != nil {
		logger.Fatal(err)
	}

	transFile := fmt.Sprintf("translations/%s.csv", cfg.lang)
	translator, err := newTranslator(transFile)
	if err != nil {
		logger.Fatal(err)
	}

	app := application{
		config:     cfg,
		logger:     logger,
		translator: translator,

		statusService: &StatusService{db: db},
		guestService:  &GuestService{db: db},
		eventService:  &EventService{db: db},
	}

	err = app.parseTemplates()
	if err != nil {
		logger.Fatal(err)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	shutdown := make(chan error)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop

		logger.Println("shutting down server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdown <- srv.Shutdown(ctx)
	}()

	logger.Printf("starting server on port %d\n", cfg.port)

	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal(err)
	}

	err = <-shutdown
	if err != nil {
		logger.Fatal(err)
	}

	logger.Println("server stopped")

	if err := db.Close(); err != nil {
		logger.Fatal(err)
	}
}

func (app *application) parseTemplates() error {
	app.templates = make(map[string]*template.Template)

	names, err := fs.Glob(assets, "html/*.html")
	if err != nil {
		return err
	}

	for _, name := range names {
		base := filepath.Base(name)
		k := strings.TrimSuffix(base, filepath.Ext(base))

		if k == "base" {
			continue
		}

		t := template.New(base).Funcs(template.FuncMap{
			"markdown":  markdown,
			"translate": app.translator.translate,
		})

		t, err = t.ParseFS(assets, name)
		if err != nil {
			return err
		}

		t, err = t.ParseFS(assets, "html/base.html")
		if err != nil {
			return err
		}

		app.templates[k] = t
	}

	return nil
}

func markdown(args ...interface{}) template.HTML {
	s := blackfriday.MarkdownCommon([]byte(fmt.Sprintf("%s", args...)))
	return template.HTML(s)
}
