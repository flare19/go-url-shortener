// internal/config/config.go
package config

import (
	"log"
	"os"
	"time"
)

type Mongo struct {
	URI        string
	DB         string
	Collection string
}

func LoadMongo() Mongo {
	return Mongo{
		URI:        MustEnv("MONGO_URI"),
		DB:         EnvOrDefault("MONGO_DB", "urlshortener"),
		Collection: EnvOrDefault("MONGO_COLLECTION", "urls"),
	}
}

func MustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}

func EnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// StatsFlushInterval returns how often StatsBuffer should flush buffered
// hit counts to the repository. Configurable via STATS_FLUSH_INTERVAL
// (e.g. "5s", "1m"), defaults to 5 seconds if unset or unparseable.
func StatsFlushInterval() time.Duration {
	raw := EnvOrDefault("STATS_FLUSH_INTERVAL", "5s")
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("config: invalid STATS_FLUSH_INTERVAL %q, defaulting to 5s: %v", raw, err)
		return 5 * time.Second
	}
	return d
}
