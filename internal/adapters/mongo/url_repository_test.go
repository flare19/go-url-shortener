package mongo

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"

	"github.com/flare19/go-url-shortener/internal/domain"
	"github.com/flare19/go-url-shortener/internal/ports"
)

// setupTestRepo spins up a real, containerized Mongo instance and returns
// a repository wired to it, plus a cleanup func the caller must defer.
func setupTestRepo(t *testing.T) (*MongoURLRepository, func()) {
	t.Helper()
	ctx := context.Background()

	mongoContainer, err := tcmongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("failed to start mongo container: %v", err)
	}

	connStr, err := mongoContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connStr))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	coll := client.Database("testdb").Collection("urls")

	if err := EnsureIndexes(ctx, coll); err != nil {
		t.Fatalf("failed to ensure indexes: %v", err)
	}

	repo := NewMongoURLRepository(coll)

	cleanup := func() {
		_ = client.Disconnect(ctx)
		_ = mongoContainer.Terminate(ctx)
	}

	return repo, cleanup
}

func TestSave_DuplicateCode_ReturnsErrDuplicateCode(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	first := &domain.URL{
		Code:      "abc1234",
		LongURL:   "https://example.com/first",
		CreatedAt: time.Now(),
	}
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("expected first save to succeed, got: %v", err)
	}

	second := &domain.URL{
		Code:      "abc1234", // same code — should collide on the unique index
		LongURL:   "https://example.com/second",
		CreatedAt: time.Now(),
	}
	err := repo.Save(ctx, second)

	if err == nil {
		t.Fatal("expected an error on duplicate code, got nil")
	}
	if err != ports.ErrDuplicateCode {
		t.Fatalf("expected ports.ErrDuplicateCode, got: %v (wrong error type — translation boundary broken)", err)
	}
}

func TestFindByCode_NotFound_ReturnsErrNotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	_, err := repo.FindByCode(ctx, "doesnotexist")

	if err == nil {
		t.Fatal("expected an error for missing code, got nil")
	}
	if err != ports.ErrNotFound {
		t.Fatalf("expected ports.ErrNotFound, got: %v (wrong error type — translation boundary broken)", err)
	}
}

func TestSaveAndFindByCode_RoundTrip(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	original := &domain.URL{
		Code:      "roundtrip1",
		LongURL:   "https://example.com/roundtrip",
		CreatedAt: time.Now().Truncate(time.Second), // avoid sub-second precision mismatches
	}

	if err := repo.Save(ctx, original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	found, err := repo.FindByCode(ctx, "roundtrip1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}

	if found.Code != original.Code || found.LongURL != original.LongURL {
		t.Fatalf("round-tripped doc mismatch: got %+v, want %+v", found, original)
	}
}
