package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	ics "github.com/arran4/golang-ical"
	"github.com/resterle/dg-cal/v2/model"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(path string) (Repo, error) {
	db, err := initDB(path)
	if err != nil {
		return Repo{}, err
	}
	return Repo{db: db}, nil
}

func initDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tournaments table
	createTournamentsTable := `
	CREATE TABLE IF NOT EXISTS tournaments (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		updated_at DATETIME NOT NULL,
		start_date DATETIME NOT NULL,
		end_date DATETIME NOT NULL,
		series TEXT NOT NULL,
		pdga_status TEXT NOT NULL,
		drating BOOLEAN NOT NULL,
		ics TEXT NOT NULL
	);`

	if _, err := db.Exec(createTournamentsTable); err != nil {
		return nil, fmt.Errorf("failed to create tournaments table: %w", err)
	}

	// Create registrations table
	createRegistrationsTable := `
	CREATE TABLE IF NOT EXISTS registrations (
		id INTEGER PRIMARY KEY,
		updated_at DATETIME NOT NULL,
		start_date DATETIME NOT NULL,
		end_date DATETIME NOT NULL,
		ics TEXT,
		FOREIGN KEY (id) REFERENCES tournaments(id)
	);`

	if _, err := db.Exec(createRegistrationsTable); err != nil {
		return nil, fmt.Errorf("failed to create registrations table: %w", err)
	}

	return db, nil
}

func (r Repo) Close() {
	r.db.Close()
}

func (r Repo) UpsertTournament(tournament *model.Tournament) error {
	if tournament == nil {
		return fmt.Errorf("Empty tournament cannot be saved")
	}
	ics, err := json.Marshal(tournament.Ics)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO tournaments (id, title, updated_at, start_date, end_date, series, pdga_status, drating, ics)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		title=excluded.title,
    	updated_at=excluded.updated_at,
     	start_date=excluded.start_date,
      	end_date=excluded.end_date,
       	series=excluded.series,
        pdga_status=excluded.pdga_status,
        drating = excluded.drating,
       	ics=excluded.ics`,
		tournament.Id, tournament.Title, tournament.UpdatedAt, tournament.StartDate, tournament.EndDate, tournament.Serie,
		tournament.PdgaStatus, tournament.DRating, string(ics))
	if err != nil {
		return err
	}
	if tournament.Registration != nil {
		return r.UpsertRegistration(tournament.Id, tournament.Registration)
	}
	return nil
}

func (r Repo) GetAllTournaments() ([]model.Tournament, error) {

	rows, err := r.db.Query(`
        SELECT tournaments.id, tournaments.title, tournaments.updated_at, tournaments.start_date, tournaments.end_date, tournaments.series,
        	tournaments.pdga_status, tournaments.drating, tournaments.ics,
        	registrations.id, registrations.updated_at, registrations.start_date, registrations.end_date, registrations.ics
        FROM tournaments LEFT JOIN registrations ON tournaments.id = registrations.id
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tournaments []model.Tournament
	for rows.Next() {
		var t model.Tournament
		var tournamentIcsJson string

		var regId sql.NullInt64
		var regUpdatedAt sql.NullTime
		var regStartDate sql.NullTime
		var regEndDate sql.NullTime
		var regIcsJson sql.NullString

		err := rows.Scan(&t.Id, &t.Title, &t.UpdatedAt, &t.StartDate, &t.EndDate, &t.Serie,
			&t.PdgaStatus, &t.DRating, &tournamentIcsJson,
			&regId, &regUpdatedAt, &regStartDate, &regEndDate, &regIcsJson)
		if err != nil {
			return nil, err
		}
		var te ics.VEvent
		err = json.Unmarshal([]byte(tournamentIcsJson), &te)
		if err != nil {
			return nil, err
		}
		t.Ics = te

		if regId.Valid {
			var re ics.VEvent
			err = json.Unmarshal([]byte(regIcsJson.String), &re)
			if err != nil {
				return nil, err
			}
			t.Registration = &model.Registration{
				UpdatedAt: regUpdatedAt.Time,
				StartDate: regStartDate.Time,
				EndDate:   regEndDate.Time,
				Ics:       re,
			}
		}

		tournaments = append(tournaments, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tournaments, nil
}

func (r Repo) UpsertRegistration(tournamentId int, registration *model.Registration) error {
	if registration == nil {
		return fmt.Errorf("Empty registration cannot be saved")
	}
	ics, err := json.Marshal(registration.Ics)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO registrations (id, updated_at, start_date, end_date, ics)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
    	updated_at=excluded.updated_at,
     	start_date=excluded.start_date,
      	end_date=excluded.end_date,
       	ics=excluded.ics`,
		tournamentId, registration.UpdatedAt, registration.StartDate, registration.EndDate, string(ics))
	return err
}
