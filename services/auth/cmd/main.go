// services/auth/cmd/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/background"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/email"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Loading config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Running migrations: %v", err)
	}
	log.Println("Database migrations complete.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Connecting to database: %v", err)
	}
	defer db.Close()

	emailSender := email.NewSender(email.NewSenderParams{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.SMTPFrom,
	})

	go background.StartUnverifiedUserCleanup(ctx, db)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Post("/internal/auth/register", handler.NewRegisterHandler(handler.RegisterHandlerParams{
		DB:          db,
		EmailSender: emailSender,
		BaseURL:     cfg.BaseURL,
	}).ServeHTTP)

	r.Get("/internal/auth/verify", handler.NewVerifyHandler(handler.VerifyHandlerParams{
		DB: db,
	}).ServeHTTP)

	r.Post("/internal/auth/rotate-key", handler.NewRotateKeyHandler(handler.RotateKeyHandlerParams{
		DB: db,
	}).ServeHTTP)

	r.Get("/internal/auth/validate", handler.NewValidateHandler(handler.ValidateHandlerParams{
		DB: db,
	}).ServeHTTP)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Auth service listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down auth service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Auth service stopped.")
}
