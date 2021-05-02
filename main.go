package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

//go:embed html/*.html migration/*.sql
var assets embed.FS

type app struct {
	db   *DB
	tmpl *template.Template
}

func main() {
	addr := flag.String("addr", ":8080", "http server address")
	dsn := flag.String("dsn", "cocorico.db", "datasource name")
	flag.Parse()

	db := NewDB(*dsn)
	if err := db.Open(); err != nil {
		log.Fatal(err)
	}

	tmpl, err := template.ParseFS(assets, "html/*.html")
	if err != nil {
		log.Fatal(err)
	}

	app := app{
		db:   db,
		tmpl: tmpl,
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
