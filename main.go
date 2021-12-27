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
	"strings"
	"time"

	"github.com/goodsign/monday"
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
	locale     string
	sessionKey string
}

type application struct {
	*bow.Core

	config config

	translator *Translator
	locale     monday.Locale
	lang       string

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
	flagSet.StringVar(&cfg.locale, "locale", "en_US", "locale of the application")
	flagSet.StringVar(&cfg.sessionKey, "session-key", "00000000000000000000000000000000", "session key for cookies encryption")

	if err := flagSet.Parse(args[1:]); err != nil {
		return err
	}

	var locale monday.Locale
	var lang string
	for _, l := range monday.ListLocales() {
		if string(l) == cfg.locale {
			locale = l
			lang = strings.Split(string(l), "_")[0]
			break
		}
	}

	if locale == "" {
		return errors.New("provided locale is in wrong format")
	}

	translator, err := NewTranslator(fmt.Sprintf("translations/%s.csv", lang))
	if err != nil {
		return err
	}

	db := bow.NewDB(cfg.dsn, fsys)
	if err := db.Open(); err != nil {
		return err
	}

	app := application{
		config:     cfg,
		translator: translator,
		locale:     locale,
		lang:       lang,

		statusService: &StatusService{db: db},
		guestService:  &GuestService{db: db},
		eventService:  &EventService{db: db},
	}

	funcs := template.FuncMap{
		"humanDate": app.humanDate,
		"humanTime": app.humanTime,
		"translate": app.translator.Translate,
	}

	app.Core, err = bow.NewCore(
		fsys,
		bow.WithSession(cfg.sessionKey),
		bow.WithGlobals(app.addGlobals),
		bow.WithFuncs(funcs),
	)
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

	if err := app.Run(srv); err != nil {
		return err
	}

	return db.Close()
}

// humanDate returns a nicely formatted string representation
// of the date from a time.Time object.
func (app *application) humanDate(t time.Time) string {
	return monday.Format(t, monday.FullFormatsByLocale[app.locale], app.locale)
}

// humanTime returns a nicely formatted string representation
// of the time from a time.Time object.
func (app *application) humanTime(t time.Time) string {
	return monday.Format(t, monday.TimeFormatsByLocale[app.locale], app.locale)
}
