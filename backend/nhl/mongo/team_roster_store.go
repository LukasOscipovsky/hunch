package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"hunch/backend/nhl/rosters"
)

const (
	databaseName         = "hunch"
	rosterCollectionName = "nhl_team_roster"
)

// rosterDocument is how a rosters.TeamRoster is stored. The team abbreviation
// is the _id, so each team has exactly one document.
type rosterDocument struct {
	Team      string    `bson:"_id"`
	Order     int       `bson:"order"`
	Roster    bson.Raw  `bson:"roster,omitempty"`
	Error     string    `bson:"error,omitempty"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

// TeamRosterStore saves team rosters to the hunch.nhl_team_roster collection.
type TeamRosterStore struct {
	coll *mongo.Collection
}

func NewTeamRosterStore(client *mongo.Client) *TeamRosterStore {
	return &TeamRosterStore{coll: client.Database(databaseName).Collection(rosterCollectionName)}
}

// SaveRosters upserts one document per team, replacing any previous roster.
// Each team's position in teams is stored so LoadRosters keeps the order.
func (s *TeamRosterStore) SaveRosters(ctx context.Context, teams []rosters.TeamRoster) error {
	if len(teams) == 0 {
		return nil
	}

	now := time.Now().UTC()
	models := make([]mongo.WriteModel, 0, len(teams))
	for i, t := range teams {
		doc := rosterDocument{Team: t.Team, Order: i, Error: t.Error, UpdatedAt: now}
		if len(t.Roster) > 0 {
			// Convert the NHL JSON to BSON so it is queryable, not an opaque blob.
			if err := bson.UnmarshalExtJSON(t.Roster, false, &doc.Roster); err != nil {
				return fmt.Errorf("convert roster %s to bson: %w", t.Team, err)
			}
		}
		models = append(models, mongo.NewReplaceOneModel().
			SetFilter(bson.D{{Key: "_id", Value: t.Team}}).
			SetReplacement(doc).
			SetUpsert(true))
	}

	if _, err := s.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
		return fmt.Errorf("save rosters: %w", err)
	}
	return nil
}

// LoadRosters returns every stored roster in the order they were saved.
func (s *TeamRosterStore) LoadRosters(ctx context.Context) ([]rosters.TeamRoster, error) {
	cur, err := s.coll.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "order", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("load rosters: %w", err)
	}
	var docs []rosterDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode rosters: %w", err)
	}

	teams := make([]rosters.TeamRoster, len(docs))
	for i, d := range docs {
		teams[i] = rosters.TeamRoster{Team: d.Team, Error: d.Error}
		if len(d.Roster) > 0 {
			// Relaxed extended JSON keeps numbers plain, matching the NHL API.
			roster, err := bson.MarshalExtJSON(d.Roster, false, false)
			if err != nil {
				return nil, fmt.Errorf("convert roster %s to json: %w", d.Team, err)
			}
			teams[i].Roster = json.RawMessage(roster)
		}
	}
	return teams, nil
}
