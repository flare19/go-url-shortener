package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/flare19/go-url-shortener/internal/ports"
	"github.com/flare19/go-url-shortener/internal/service"
)

// statsResponse is the JSON shape returned by GET /{code}/stats,
// matching the contract documented in README.md and docs/api-reference.md.
type statsResponse struct {
	Code      string `json:"code"`
	LongURL   string `json:"long_url"`
	HitCount  int64  `json:"hit_count"`
	CreatedAt string `json:"created_at"`
}

// writeStatsError writes a JSON error body. This intentionally diverges
// from redirectHandler's plain-text http.NotFound/http.Error pattern —
// see ADR 0006. redirectHandler stays untouched.
func writeStatsError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func statsHandler(svc *service.URLService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		code := vars["code"]

		u, err := svc.Get(r.Context(), code)
		if err != nil {
			switch {
			case errors.Is(err, ports.ErrNotFound):
				writeStatsError(w, http.StatusNotFound, "short url not found")
			default:
				log.Printf("stats error: %v", err)
				writeStatsError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		resp := statsResponse{
			Code:      u.Code,
			LongURL:   u.LongURL,
			HitCount:  u.HitCounter,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
