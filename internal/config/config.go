// internal/config/config.go
package config

import (
	"log"
	"os"
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
