package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/golangcollege/sessions"
	"github.com/russross/blackfriday"
)

//go:embed views/layouts/*.html
//go:embed views/events/*.html
//go:embed views/guests/*.html
//go:embed views/statuses/*.html
//go:embed migrations/*.sql
//go:embed translations/*.csv
var assets embed.FS

var (
	ErrNoRecord       = errors.New("no record")
	ErrDuplicateEmail = errors.New("duplicate email")
	ErrStatusUsed     = errors.New("status used")
)

type config struct {
	port       int
	dsn        string
	lang       string
	sessionKey string
}

type application struct {
	config config
	logger *log.Logger

	views map[string]view

	translator *translator
	session    *sessions.Session

	statusService *StatusService
	guestService  *GuestService
	eventService  *EventService
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 8080, "http server port")
	flag.StringVar(&cfg.dsn, "dsn", "tdispo.db", "database datasource name")
	flag.StringVar(&cfg.lang, "lang", "en", "language of the application")
	flag.StringVar(&cfg.sessionKey, "session-key", "0g6kFh15VxjIfRSDDoXxrK2DLivlX6xt", "session key for cookies encryption")
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

	session := sessions.New([]byte(cfg.sessionKey))
	session.Lifetime = 12 * time.Hour

	app := application{
		config:     cfg,
		logger:     logger,
		translator: translator,
		session:    session,

		statusService: &StatusService{db: db},
		guestService:  &GuestService{db: db},
		eventService:  &EventService{db: db},
	}

	funcs := template.FuncMap{
		"markdown":  markdown,
		"translate": app.translator.translate,
	}

	app.views, err = parseViews(assets, "views", funcs)
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

func markdown(args ...interface{}) template.HTML {
	s := blackfriday.MarkdownCommon([]byte(fmt.Sprintf("%s", args...)))
	return template.HTML(s)
}
