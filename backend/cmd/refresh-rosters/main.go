// Command refresh-rosters fetches every team's current roster from the NHL
// API and saves it to Mongo, the same as the server's daily refresh. It does
// nothing if Mongo already holds rosters.
package main

import (
	"context"
	"log"
	"time"

	"hunch/backend/nhl"
	"hunch/backend/nhl/mongo"
	"hunch/backend/nhl/rosters"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, mongo.URIFromEnv())
	if err != nil {
		log.Fatal(err)
	}
	defer mongoClient.Disconnect(context.Background())

	service := rosters.NewService(nhl.NewClient(), mongo.NewTeamRosterStore(mongoClient))
	refreshed, err := service.RefreshIfEmpty(ctx)
	if err != nil {
		log.Fatalf("refresh rosters: %v", err)
	}
	if !refreshed {
		log.Println("rosters already stored, skipping refresh")
		return
	}
	log.Println("rosters refreshed")
}
