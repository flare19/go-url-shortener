// cmd/redirector/main.go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/flare19/go-url-shortener/internal/adapters/encoding"
	"github.com/flare19/go-url-shortener/internal/adapters/memcache"
	mongoadapter "github.com/flare19/go-url-shortener/internal/adapters/mongo"
	"github.com/flare19/go-url-shortener/internal/adapters/stats"
	"github.com/flare19/go-url-shortener/internal/config"
	"github.com/flare19/go-url-shortener/internal/httpmiddleware"
	"github.com/flare19/go-url-shortener/internal/ports"
	"github.com/flare19/go-url-shortener/internal/service"
	"github.com/joho/godotenv"
)

type appConfig struct {
	mongo      config.Mongo
	listenAddr string
}

func loadConfig() appConfig {
	return appConfig{
		mongo:      config.LoadMongo(),
		listenAddr: config.EnvOrDefault("LISTEN_ADDR", ":8081"),
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

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.mongo.URI))
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}
	coll := client.Database(cfg.mongo.DB).Collection(cfg.mongo.Collection)

	repo := mongoadapter.NewMongoURLRepository(coll)
	encoder := encoding.NewRandomEncoder() // redirector doesn't use this — see note below
	cache := memcache.New()

	svc := service.NewURLService(repo, encoder, cache)

	statsBuffer := stats.NewStatsBuffer(repo.IncrementHitsBatch, config.StatsFlushInterval())
	statsBuffer.Start(context.Background())

	router := mux.NewRouter()
	router.Use(httpmiddleware.CORS(config.CORSAllowedOrigin()))
	router.HandleFunc("/healthz", healthHandler).Methods(http.MethodGet)
	router.HandleFunc("/{code}/stats", statsHandler(svc)).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/{code}", redirectHandler(svc, statsBuffer)).Methods(http.MethodGet)

	srv := &http.Server{
		Addr:         cfg.listenAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("redirector listening on %s", cfg.listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// execution continues here immediately — the goroutine runs concurrently,
	// main() doesn't wait for it, it falls straight through to this line:

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	statsBuffer.Stop(shutdownCtx)
	if err := client.Disconnect(shutdownCtx); err != nil {
		log.Printf("mongo disconnect error: %v", err)
	}
}

func redirectHandler(svc *service.URLService, sb *stats.StatsBuffer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		code := vars["code"]
		u, err := svc.Get(r.Context(), code)
		if err != nil {
			switch {
			case errors.Is(err, ports.ErrNotFound):
				http.NotFound(w, r)
			default:
				log.Printf("redirect error: %v", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}
		sb.RecordHit(code)
		http.Redirect(w, r, u.LongURL, http.StatusFound)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
