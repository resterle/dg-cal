package main

import (
	"fmt"
	"log"

	"github.com/resterle/dg-cal/v2/db"
	"github.com/resterle/dg-cal/v2/gto"
	"github.com/resterle/dg-cal/v2/model"

	// Import sqlite3 driver for database/sql - registers itself via init()
	_ "github.com/mattn/go-sqlite3"
)

var tournaments map[int]model.Tournament
var repo db.Repo

func main() {
	tournaments = make(map[int]model.Tournament)

	var err error
	repo, err = db.NewRepo("events.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer repo.Close()

	fmt.Printf("Database initialized successfully %+v\n", repo)
	fmt.Println("Tables created: tournaments, registrations")
	fmt.Println()

	/*
		gtoEvents, err := gto.FetchTournaments()
		fmt.Printf("\nGTO %s --> %v\n", err, len(gtoEvents))

		ttt, err := repo.GetAllTournaments()
		fmt.Printf("DB %s --> %v\n", err, len(ttt))
	*/
	fmt.Printf("LOAD %s\n", load())
	fmt.Printf("SYNC %s\n", sync())
}

func sync() error {
	log.Printf("SYNC")
	gtoTournaments, err := gto.FetchTournaments()
	if err != nil {
		return err
	}
	for _, gtoT := range gtoTournaments {
		storedT := tournaments[gtoT.Id]
		if storedT.UpdatedAt.Before(gtoT.UpdatedAt) {
			log.Printf("Info: updating %d", gtoT.Id)
			details, err := gto.FetchEventDetails(gtoT.Id)
			if err != nil {
				log.Printf("Error: loading %d failed", gtoT.Id)
				return err
			}
			gtoT.Serie = details.Series
			gtoT.PdgaStatus = details.PDGAStatus
			gtoT.DRating = details.DRatingConsideration
			tournaments[gtoT.Id] = *gtoT
			repo.UpsertTournament(gtoT)
		} else if storedT.Registration != nil && gtoT.Registration != nil && storedT.Registration.UpdatedAt.Before(gtoT.Registration.UpdatedAt) {
			repo.UpsertRegistration(gtoT.Id, gtoT.Registration)
		}
	}
	return nil
}

func load() error {
	log.Printf("LOAD")
	t, err := repo.GetAllTournaments()
	if err != nil {
		return err
	}
	for _, tt := range t {
		tournaments[tt.Id] = tt
	}
	return nil
}
