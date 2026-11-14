package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"render-projects/internal/formulawar"
	"render-projects/internal/secrethitler"
	"render-projects/internal/undercover"
)

//go:embed web/*
var web embed.FS

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/formula-war", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/formula-war/", http.StatusMovedPermanently)
	})
	mux.Handle("/formula-war/", http.StripPrefix("/formula-war", formulawar.Handler("/formula-war")))
	mux.HandleFunc("/undercover", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/undercover/", http.StatusMovedPermanently)
	})
	mux.Handle("/undercover/", http.StripPrefix("/undercover", undercover.Handler()))
	mux.HandleFunc("/secret-hitler", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/secret-hitler/", http.StatusMovedPermanently)
	})
	mux.Handle("/secret-hitler/", http.StripPrefix("/secret-hitler", secrethitler.Handler()))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	files, _ := fs.Sub(web, "web")
	static := http.FileServer(http.FS(files))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/style.css" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; frame-ancestors 'none'")
		static.ServeHTTP(w, r)
	})
	return mux
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	log.Printf("render listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
