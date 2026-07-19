// cmd/writer/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/flare19/go-url-shortener/internal/adapters/encoding"
	"github.com/flare19/go-url-shortener/internal/adapters/memcache"
	mongoadapter "github.com/flare19/go-url-shortener/internal/adapters/mongo"
	cfgpkg "github.com/flare19/go-url-shortener/internal/config"
	"github.com/flare19/go-url-shortener/internal/domain"
	"github.com/flare19/go-url-shortener/internal/service"
)

type appConfig struct {
	mongoURI          string
	mongoDB           string
	mongoColl         string
	listenAddr        string
	redirectorBaseURL string
}

func loadConfig() appConfig {
	return appConfig{
		mongoURI:          cfgpkg.MustEnv("MONGO_URI"),
		mongoDB:           cfgpkg.EnvOrDefault("MONGO_DB", "urlshortener"),
		mongoColl:         cfgpkg.EnvOrDefault("MONGO_COLLECTION", "urls"),
		listenAddr:        cfgpkg.EnvOrDefault("LISTEN_ADDR", ":8080"),
		redirectorBaseURL: cfgpkg.EnvOrDefault("REDIRECTOR_BASE_URL", "http://localhost:8081"),
	}
}

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	if err := godotenv.Load(".env." + env); err != nil {
		log.Printf("no .env.%s file found, relying on real environment variables", env)
	}

	cfg := loadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.mongoURI))
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}
	coll := client.Database(cfg.mongoDB).Collection(cfg.mongoColl)

	if err := mongoadapter.EnsureIndexes(ctx, coll); err != nil {
		log.Fatalf("failed to ensure indexes: %v", err)
	}

	repo := mongoadapter.NewMongoURLRepository(coll)
	encoder := encoding.NewRandomEncoder()
	cache := memcache.New()

	svc := service.NewURLService(repo, encoder, cache)

	router := mux.NewRouter()
	router.HandleFunc("/shorten", createHandler(svc, cfg.redirectorBaseURL)).Methods(http.MethodPost)
	router.HandleFunc("/healthz", healthHandler).Methods(http.MethodGet)

	srv := &http.Server{
		Addr:         cfg.listenAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("writer listening on %s", cfg.listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	if err := client.Disconnect(shutdownCtx); err != nil {
		log.Printf("mongo disconnect error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type createRequest struct {
	LongURL string `json:"long_url"`
}

type createResponse struct {
	Code      string `json:"code"`
	ShortURL  string `json:"short_url"`
	LongURL   string `json:"long_url"`
	CreatedAt string `json:"created_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func createHandler(svc *service.URLService, redirectorBaseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		u, err := svc.Create(r.Context(), req.LongURL)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrInvalidURL):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				// covers retry-exhaustion and any repo/encoder failure —
				// confirm with OpenCode whether retry exhaustion surfaces as a
				// distinct sentinel; if so, add a case above for a sharper status
				log.Printf("create error: %v", err)
				writeError(w, http.StatusInternalServerError, "failed to create short url")
			}
			return
		}

		resp := createResponse{
			Code:      u.Code,
			ShortURL:  redirectorBaseURL + "/" + u.Code,
			LongURL:   u.LongURL,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}
