package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/handler"
)

type NewParams struct {
	WebhookHandler *handler.WebhookHandler
}

func New(params NewParams) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Post("/webhooks/recallai", params.WebhookHandler.HandleRecallWebhook)

	return r
}
