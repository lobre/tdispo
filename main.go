package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golangcollege/sessions"
	"github.com/lobre/tdispo/bow"
)

const (
	layoutDatetime = "2006-01-02 15:04"
	layoutDate     = "2006-01-02"
	layoutTime     = "15:04"
)

//go:embed views/layouts/*.html
//go:embed views/events/*.html
//go:embed views/guests/*.html
//go:embed views/statuses/*.html
//go:embed migrations/*.sql
//go:embed translations/*.csv
//go:embed static
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

	translator *Translator
	session    *sessions.Session
	views      bow.Views

	statusService *StatusService
	guestService  *GuestService
	eventService  *EventService
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	var cfg config

	flagSet := flag.NewFlagSet(args[0], flag.ExitOnError)

	flagSet.IntVar(&cfg.port, "port", 8080, "http server port")
	flagSet.StringVar(&cfg.dsn, "dsn", "tdispo.db", "database data source name")
	flagSet.StringVar(&cfg.lang, "lang", "en", "language of the application")
	flagSet.StringVar(&cfg.sessionKey, "session-key", "0g6kFh15VxjIfRSDDoXxrK2DLivlX6xt", "session key for cookies encryption")

	if err := flagSet.Parse(args[1:]); err != nil {
		return err
	}

	db := bow.NewDB(cfg.dsn, assets)
	if err := db.Open(); err != nil {
		return err
	}

	translator, err := NewTranslator(fmt.Sprintf("translations/%s.csv", cfg.lang))
	if err != nil {
		return err
	}

	session := sessions.New([]byte(cfg.sessionKey))
	session.Lifetime = 12 * time.Hour

	app := application{
		config:     cfg,
		translator: translator,
		session:    session,

		statusService: &StatusService{db: db},
		guestService:  &GuestService{db: db},
		eventService:  &EventService{db: db},
	}

	funcs := template.FuncMap{
		"humanDate": humanDate,
		"translate": app.translator.Translate,
	}

	err = app.views.Parse(assets, "views", funcs, app.addDefaultData)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := bow.Run(srv); err != nil {
		return err
	}

	return db.Close()
}

// humanDate returns a nicely formatted string representation
// of a time.Time object.
func humanDate(t time.Time) string {
	return t.Format("02 Jan 2006 at 15:04")
}
