package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/flare19/go-url-shortener/internal/domain"
	"github.com/flare19/go-url-shortener/internal/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mongoURLDoc mirrors domain.URL for BSON (de)serialization.
// Kept separate from domain.URL so persistence concerns (tags)
// never leak into the domain type.
type mongoURLDoc struct {
	Code       string    `bson:"code"`
	LongURL    string    `bson:"long_url"`
	CreatedAt  time.Time `bson:"created_at"`
	HitCounter int64     `bson:"hit_count"` // was hit_counter — schema doc says hit_count
}

func toDoc(u *domain.URL) mongoURLDoc {
	return mongoURLDoc{
		Code:       u.Code,
		LongURL:    u.LongURL,
		CreatedAt:  u.CreatedAt,
		HitCounter: u.HitCounter,
	}
}

func (d mongoURLDoc) toDomain() *domain.URL {
	return &domain.URL{
		Code:       d.Code,
		LongURL:    d.LongURL,
		CreatedAt:  d.CreatedAt,
		HitCounter: d.HitCounter,
	}
}

// MongoURLRepository implements ports.URLRepository against a Mongo collection.
type MongoURLRepository struct {
	coll *mongo.Collection
}

// NewMongoURLRepository wires a repository to an already-connected collection.
// Connection lifecycle (client creation, ping, disconnect) is cmd's job, not this adapter's.
func NewMongoURLRepository(coll *mongo.Collection) *MongoURLRepository {
	return &MongoURLRepository{coll: coll}
}

// compile-time interface check — same idiom as base62.go
var _ ports.URLRepository = (*MongoURLRepository)(nil)

// EnsureIndexes creates the unique index on `code`. Call once at startup (cmd), not per-request.
func EnsureIndexes(ctx context.Context, coll *mongo.Collection) error {
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (r *MongoURLRepository) Save(ctx context.Context, u *domain.URL) error {
	_, err := r.coll.InsertOne(ctx, toDoc(u))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ports.ErrDuplicateCode
		}
		return err
	}
	return nil
}

func (r *MongoURLRepository) FindByCode(ctx context.Context, code string) (*domain.URL, error) {
	var doc mongoURLDoc
	err := r.coll.FindOne(ctx, bson.M{"code": code}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *MongoURLRepository) IncrementHitsBatch(ctx context.Context, deltas map[string]int64) error {
	if len(deltas) == 0 {
		return nil
	}

	models := make([]mongo.WriteModel, 0, len(deltas))
	for code, delta := range deltas {
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"code": code}).
			SetUpdate(bson.M{"$inc": bson.M{"hit_count": delta}}), // was hit_counter
		)
	}

	_, err := r.coll.BulkWrite(ctx, models)
	return err
}
