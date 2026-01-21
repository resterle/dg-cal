package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/resterle/dg-cal/v2/db"
	"github.com/resterle/dg-cal/v2/gto"
	"github.com/resterle/dg-cal/v2/service"
	"github.com/resterle/dg-cal/v2/web"

	// Import sqlite3 driver for database/sql - registers itself via init()
	_ "modernc.org/sqlite"
)

var repo *db.Repo
var ticker *time.Ticker

func main() {
	var err error

	sessionId := os.Getenv("SESSION_ID")
	if sessionId == "" {
		panic("SESSION_ID missing")
	}

	loginData := os.Getenv("LOGIN_DATA")
	if loginData == "" {
		panic("LOGIN_DATA missing")
	}

	syncIntervalS := os.Getenv("SYNC_INTERVAL")
	if syncIntervalS == "" {
		syncIntervalS = "30"
	}
	syncIntervalInMinutes, err := strconv.Atoi(syncIntervalS)
	if err != nil {
		panic("sync interval " + err.Error())
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		panic("DB_PATH missing")
	}

	dbDir := filepath.Dir(dbPath)
	err = os.MkdirAll(dbDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	repo, err = db.NewRepo(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer repo.Close()

	gtoService := gto.NewGtoService(sessionId, loginData)

	tournamentService, err := service.NewTournamentService(repo, &gtoService)
	if err != nil {
		panic(err)
	}
	calendarservice := service.NewCalendarService(repo)

	icsService := service.NewIcsService(calendarservice, tournamentService)

	syncInterval := time.Minute * time.Duration(syncIntervalInMinutes)
	ticker = time.NewTicker(syncInterval)
	defer ticker.Stop()
	go scheduler(tournamentService, syncIntervalInMinutes)

	webApp := web.NewWebApp(tournamentService, calendarservice, icsService, syncInterval)

	http.HandleFunc("GET /{$}", webApp.WelcomeHandler)
	http.HandleFunc("GET /tournaments", webApp.TournamentsHandler)
	http.HandleFunc("GET /tournament/{id}", webApp.TournamentDetailHandler)
	http.HandleFunc("GET /registrations", webApp.RegistrationsHandler)
	http.HandleFunc("GET /calendar/new", webApp.CreateCalendarFormHandler)
	http.HandleFunc("POST /calendar/create", webApp.CreateCalendarHandler)
	http.HandleFunc("GET /calendar/created", webApp.CalendarCreatedHandler)
	http.HandleFunc("GET /calendar/edit", webApp.AccessCalendarFormHandler)
	http.HandleFunc("POST /calendar/edit", webApp.AccessCalendarHandler)
	http.HandleFunc("GET /calendar/edit/{id}", webApp.EditCalendarFormHandler)
	http.HandleFunc("POST /calendar/edit/{id}", webApp.EditCalendarHandler)
	http.HandleFunc("GET /api/tournaments", webApp.TournamentHandler)
	http.HandleFunc("GET /ical/{id}", webApp.IcsHandler)
	http.HandleFunc("GET /common.css", webApp.CommonCSSHandler)
	http.HandleFunc("GET /table-filters.js", webApp.TableFiltersJSHandler)
	http.HandleFunc("GET /fonts/{name}", webApp.FontHandler)

	http.HandleFunc("GET /admin", webApp.AdminHandler)
	http.HandleFunc("POST /admin/calendar/delete/{id}", webApp.DeleteCalendarHandler)
	http.HandleFunc("GET /admin/calendar/{id}", webApp.AdminViewCalendarHandler)
	http.HandleFunc("POST /admin/calendar/{id}", webApp.AdminUpdateCalendarHandler)
	http.HandleFunc("GET /admin/tournaments", webApp.AdminTournamentsHandler)
	http.HandleFunc("GET /admin/tournament/{id}/history", webApp.AdminTournamentHistoryHandler)

	http.HandleFunc("/", webApp.NotFoundHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Web service starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, web.LoggingMiddleware(http.DefaultServeMux)))

}

func scheduler(s *service.TournamentService, syncInterval int) {
	for {
		s.Sync()
		log.Printf("Next sync in %d minutes", syncInterval)
		<-ticker.C
	}
}
