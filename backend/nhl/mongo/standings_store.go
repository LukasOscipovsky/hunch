package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	standingsCollectionName = "nhl_standings"
	// standingsID is the _id of the single document holding the latest
	// standings. Each refresh overwrites it.
	standingsID = "current"
)

// standingsDocument is how the standings are stored.
type standingsDocument struct {
	ID        string    `bson:"_id"`
	Standings bson.Raw  `bson:"standings"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

// StandingsStore saves the NHL standings to the hunch.nhl_standings collection.
type StandingsStore struct {
	coll *mongo.Collection
}

func NewStandingsStore(client *mongo.Client) *StandingsStore {
	return &StandingsStore{coll: client.Database(databaseName).Collection(standingsCollectionName)}
}

// SaveStandings replaces the stored standings with the NHL API JSON.
func (s *StandingsStore) SaveStandings(ctx context.Context, standings json.RawMessage) error {
	doc := standingsDocument{ID: standingsID, UpdatedAt: time.Now().UTC()}
	// Convert the NHL JSON to BSON so it is queryable, not an opaque blob.
	if err := bson.UnmarshalExtJSON(standings, false, &doc.Standings); err != nil {
		return fmt.Errorf("convert standings to bson: %w", err)
	}
	_, err := s.coll.ReplaceOne(ctx, bson.D{{Key: "_id", Value: standingsID}}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("save standings: %w", err)
	}
	return nil
}

// LoadStandings returns the stored standings as JSON, or nil if none are
// stored.
func (s *StandingsStore) LoadStandings(ctx context.Context) (json.RawMessage, error) {
	var doc standingsDocument
	err := s.coll.FindOne(ctx, bson.D{{Key: "_id", Value: standingsID}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load standings: %w", err)
	}
	// Relaxed extended JSON keeps numbers plain, matching the NHL API.
	standings, err := bson.MarshalExtJSON(doc.Standings, false, false)
	if err != nil {
		return nil, fmt.Errorf("convert standings to json: %w", err)
	}
	return standings, nil
}
