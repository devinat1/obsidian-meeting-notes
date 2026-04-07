package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/handler"
)

type NewParams struct {
	BotStatusHandler *handler.BotStatusHandler
	DeleteBotHandler *handler.DeleteBotHandler
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

	r.Route("/internal/bots/{botId}", func(r chi.Router) {
		r.Get("/status", params.BotStatusHandler.GetBotStatus)
		r.Delete("/", params.DeleteBotHandler.DeleteBot)
	})

	return r
}
