package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

//go:embed html/*.html migration/*.sql
var assets embed.FS

type app struct {
	ts map[string]*template.Template // template set

	statusService *StatusService
	guestService  *GuestService
	eventService  *EventService
}

func main() {
	addr := flag.String("addr", ":8080", "http server address")
	dsn := flag.String("dsn", "cocorico.db", "datasource name")
	flag.Parse()

	db := NewDB(*dsn)
	if err := db.Open(); err != nil {
		log.Fatal(err)
	}

	ts, err := parseTemplates()
	if err != nil {
		log.Fatal(err)
	}

	app := app{
		ts: ts,

		statusService: &StatusService{db: db},
		guestService:  &GuestService{db: db},
		eventService:  &EventService{db: db},
	}

	srv := &http.Server{
		Addr:    *addr,
		Handler: app.routes(),
	}

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)

	go func() {
		log.Printf("starting server on %s\n", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-stop

	log.Println("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("server down")

	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
}

func parseTemplates() (map[string]*template.Template, error) {
	ts := make(map[string]*template.Template)

	names, err := fs.Glob(assets, "html/*.html")
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		base := filepath.Base(name)
		k := strings.TrimSuffix(base, filepath.Ext(base))

		if k == "base" {
			continue
		}

		t, err := template.ParseFS(assets, name)
		if err != nil {
			return nil, err
		}

		t, err = t.ParseFS(assets, "html/base.html")
		if err != nil {
			return nil, err
		}

		ts[k] = t
	}

	return ts, nil
}
