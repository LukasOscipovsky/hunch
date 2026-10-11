package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"hunch/backend/nhl"
	"hunch/backend/nhl/mongo"
	"hunch/backend/nhl/rosters"
	"hunch/backend/nhl/standings"
)

func main() {
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	mongoClient, err := mongo.Connect(connectCtx, mongo.URIFromEnv())
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	defer mongoClient.Disconnect(context.Background())

	nhlClient := nhl.NewClient()
	rosterService := rosters.NewService(nhlClient, mongo.NewTeamRosterStore(mongoClient))
	go rosterService.RunDaily(context.Background())
	standingsService := standings.NewService(nhlClient, mongo.NewStandingsStore(mongoClient))
	go standingsService.RunDaily(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Hello World from Go!"})
	})
	mux.HandleFunc("GET /api/standings", func(w http.ResponseWriter, r *http.Request) {
		body, err := standingsService.CurrentStandings(r.Context())
		if err != nil {
			log.Printf("standings: %v", err)
			http.Error(w, "failed to load NHL standings", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
	mux.HandleFunc("GET /api/rosters", func(w http.ResponseWriter, r *http.Request) {
		teams, err := rosterService.CurrentRosters(r.Context())
		if err != nil {
			log.Printf("rosters: %v", err)
			http.Error(w, "failed to load NHL rosters", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(teams)
	})

	log.Println("Backend listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
