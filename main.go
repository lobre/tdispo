package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lobre/tdispo/bow"
)

//go:generate tailwindcss --input ./tailwind.css --output ./assets/tailwind.css --minify

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
//go:embed assets
var fsys embed.FS

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
	*bow.Core

	config config

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
	flagSet.StringVar(&cfg.lang, "lang", "auto", "language of the application as BCP 47")
	flagSet.StringVar(&cfg.sessionKey, "session-key", "xxx", "session key for cookies encryption")

	if err := flagSet.Parse(args[1:]); err != nil {
		return err
	}

	app := application{
		config: cfg,
	}

	var err error

	app.Core, err = bow.NewCore(
		fsys,
		bow.WithDB(cfg.dsn),
		bow.WithSession(cfg.sessionKey),
		bow.WithGlobals(app.addGlobals),
		bow.WithTranslator(cfg.lang),
	)
	if err != nil {
		return err
	}

	app.statusService = &StatusService{db: app.DB}
	app.guestService = &GuestService{db: app.DB}
	app.eventService = &EventService{db: app.DB}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := app.Run(srv); err != nil {
		return err
	}

	return app.DB.Close()
}
