package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/resterle/dg-cal/v2/model"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(path string) (*Repo, error) {
	db, err := initDB(path)
	if err != nil {
		return nil, err
	}
	return &Repo{db: db}, nil
}

func initDB(dbPath string) (*sql.DB, error) {
	connectionString := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)", dbPath)
	db, err := sql.Open("sqlite", connectionString)
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
		status TEXT NOT NULL,
		location string,
		geo_location string,
		updated_at DATETIME NOT NULL,
		start_date DATETIME NOT NULL,
		end_date DATETIME NOT NULL,
		series TEXT NOT NULL,
		pdga_tier TEXT NOT NULL,
		pdga_id TEXT NOT NULL,
		drating BOOLEAN NOT NULL
	);`

	if _, err := db.Exec(createTournamentsTable); err != nil {
		return nil, fmt.Errorf("failed to create tournaments table: %w", err)
	}

	// Create registrations table
	createRegistrationsTable := `
	CREATE TABLE IF NOT EXISTS registrations (
		id INTEGER,
		title TEXT NOT NULL,
		start_date DATETIME NOT NULL,
		end_date DATETIME NOT NULL,
		FOREIGN KEY (id) REFERENCES tournaments(id),
		UNIQUE(id, title)
	);`

	if _, err := db.Exec(createRegistrationsTable); err != nil {
		return nil, fmt.Errorf("failed to create registrations table: %w", err)
	}

	// Create calendars table
	createCalendarsTable := `
	CREATE TABLE IF NOT EXISTS calendars (
		id TEXT PRIMARY KEY,
		edit_id TEXT NOT NULL,
		title TEXT NOT NULL,
		email TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		subscription_config TEXT NOT NULL,
		retrieved_at DATETIME
	) WITHOUT ROWID;`

	if _, err := db.Exec(createCalendarsTable); err != nil {
		return nil, fmt.Errorf("failed to create calendars table: %w", err)
	}

	// Create subscriptions table
	createSubscriptionsTable := `
	CREATE TABLE IF NOT EXISTS subscriptions (
		calendar_id TEXT NOT NULL,
		tournament_id INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (calendar_id) REFERENCES calendars(id),
		FOREIGN KEY (tournament_id) REFERENCES tournaments(id),
		UNIQUE(calendar_id, tournament_id)
	);`

	if _, err := db.Exec(createSubscriptionsTable); err != nil {
		return nil, fmt.Errorf("failed to create subscriptions table: %w", err)
	}

	// Create subscriptions table
	createHistoryTable := `
	CREATE TABLE IF NOT EXISTS tournament_history (
		tournament_id INTEGER NOT NULL,
		date DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		snapshot TEXT NOT NULL,
		FOREIGN KEY (tournament_id) REFERENCES tournaments(id),
		UNIQUE(tournament_id, date)
	);`

	if _, err := db.Exec(createHistoryTable); err != nil {
		return nil, fmt.Errorf("failed to create subscriptions table: %w", err)
	}

	return db, nil
}

func (r *Repo) Close() {
	r.db.Close()
}

func (r *Repo) UpsertTournament(tournament *model.Tournament) error {
	if tournament == nil {
		return fmt.Errorf("Empty tournament cannot be saved")
	}

	series, err := json.Marshal(tournament.Series)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO tournaments (id, title, status, location, geo_location, updated_at, start_date, end_date, series, pdga_tier, pdga_id, drating)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		title=excluded.title,
		location=excluded.location,
		status=excluded.status,
		geo_location=excluded.geo_location,
    	updated_at=excluded.updated_at,
     	start_date=excluded.start_date,
      	end_date=excluded.end_date,
       	series=excluded.series,
        pdga_tier=excluded.pdga_tier,
        pdga_id=excluded.pdga_id,
        drating = excluded.drating`,
		tournament.Id, tournament.Title, tournament.Status, tournament.Localtion, tournament.GeoLocation, tournament.UpdatedAt, tournament.StartDate, tournament.EndDate, string(series),
		tournament.PdgaTier, tournament.PdgaId, tournament.DRating)
	if err != nil {
		return err
	}

	for _, registration := range tournament.Registrations {
		if err := r.UpsertRegistration(tournament.Id, registration); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repo) GetAllTournaments() ([]model.Tournament, error) {
	rows, err := r.db.Query(`
        SELECT id, title, status, location, geo_location, updated_at, start_date, end_date, series, pdga_tier, pdga_id, drating
        FROM tournaments
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tournaments []model.Tournament
	for rows.Next() {
		var t model.Tournament
		var seriesJson string

		err := rows.Scan(&t.Id, &t.Title, &t.Status, &t.Localtion, &t.GeoLocation, &t.UpdatedAt, &t.StartDate, &t.EndDate, &seriesJson,
			&t.PdgaTier, &t.PdgaId, &t.DRating)

		if err != nil {
			return nil, err
		}

		var series []string
		err = json.Unmarshal([]byte(seriesJson), &series)
		if err != nil {
			return nil, err
		}
		t.Series = series

		registrations, err := r.getRegistrations(t.Id)
		if err != nil {
			return nil, err
		}
		t.Registrations = registrations

		tournaments = append(tournaments, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tournaments, nil
}

func (r *Repo) GetCalendarUpdateCount() (map[int]int, error) {
	rows, err := r.db.Query("SELECT tournament_id, count(*) FROM tournament_history GROUP BY tournament_id")
	if err != nil {
		return map[int]int{}, err
	}
	defer rows.Close()

	result := make(map[int]int, 100)
	var id int
	var count int
	for rows.Next() {
		err := rows.Scan(&id, &count)
		if err != nil {
			return map[int]int{}, err
		}
		result[id] = count
	}
	return result, nil
}

func (r *Repo) GetTournamentHistory(id int) ([]*model.Tournament, error) {
	rows, err := r.db.Query(`
		SELECT snapshot FROM tournament_history
		WHERE tournament_id = ?`, id)
	if err != nil {
		return []*model.Tournament{}, err
	}
	defer rows.Close()

	result := []*model.Tournament{}
	for rows.Next() {
		var jsonSnapshot string
		err := rows.Scan(&jsonSnapshot)
		if err != nil {
			return []*model.Tournament{}, err
		}

		var t model.Tournament
		err = json.Unmarshal([]byte(jsonSnapshot), &t)
		if err != nil {
			return []*model.Tournament{}, err
		}

		result = append(result, &t)
	}

	return result, nil
}

func (r *Repo) getRegistrations(tournamentId int) ([]*model.Registration, error) {
	result := []*model.Registration{}

	rows, err := r.db.Query(`
        SELECT title, start_date, end_date
        FROM registrations WHERE id = ?
    `, tournamentId)

	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		r := model.Registration{}
		rows.Scan(&r.Title, &r.StartDate, &r.EndDate)
		result = append(result, &r)
	}

	return result, nil
}

func (r *Repo) UpsertRegistration(tournamentId int, registration *model.Registration) error {
	if registration == nil {
		return fmt.Errorf("Empty registration cannot be saved")
	}
	_, err := r.db.Exec(`
		INSERT INTO registrations (id, title, start_date, end_date)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(id, title) DO UPDATE SET
     	start_date=excluded.start_date,
      	end_date=excluded.end_date,
       	title=excluded.title`,
		tournamentId, registration.Title, registration.StartDate, registration.EndDate)
	return err
}

func (r *Repo) GetCalendars() ([]*model.Calendar, error) {
	rows, err := r.db.Query(`
        SELECT id, title, email, created_at, updated_at, subscription_config, retrieved_at
        FROM calendars
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calendars []*model.Calendar
	for rows.Next() {
		var c model.Calendar
		var configJson string

		err := rows.Scan(&c.Id, &c.Title, &c.Email, &c.CreatedAt, &c.UpdatedAt, &configJson, &c.RetrievedAt)

		if err != nil {
			return nil, err
		}

		var config model.SubscriptionConfig
		err = json.Unmarshal([]byte(configJson), &config)
		if err != nil {
			return nil, err
		}
		c.Config = &config

		calendars = append(calendars, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return calendars, nil
}

func (r *Repo) CreateCalendar(id, editId, title string, config model.SubscriptionConfig) error {
	subscriptionConfigJson, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO calendars (id, edit_id, title, email, created_at, updated_at, subscription_config)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		id, editId, title, "", time.Now(), time.Now(), string(subscriptionConfigJson))

	return err
}

func (r *Repo) GetCalendarById(id string) (*model.Calendar, error) {
	return r.getCalendar("id", id)
}

func (r *Repo) GetCalendarByEditId(editId string) (*model.Calendar, error) {
	return r.getCalendar("edit_id", editId)
}

func (r *Repo) getCalendar(idColumn string, id string) (*model.Calendar, error) {
	query := fmt.Sprintf(`
		SELECT id, title, email, created_at, updated_at, subscription_config, retrieved_at
		FROM calendars WHERE %s = ?`, idColumn)
	rows, err := r.db.Query(query, id)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var subscriptionConfigJson sql.NullString
		c := model.Calendar{Config: &model.SubscriptionConfig{Tournaments: []int{}, Series: []string{}}}
		rows.Scan(&c.Id, &c.Title, &c.Email, &c.CreatedAt, &c.UpdatedAt, &subscriptionConfigJson, &c.RetrievedAt)

		if subscriptionConfigJson.Valid {
			if err := json.Unmarshal([]byte(subscriptionConfigJson.String), c.Config); err != nil {
				return nil, err
			}
		}
		return &c, nil
	}
	return nil, nil
}

func (r *Repo) UpdateCalendar(calendar *model.Calendar) error {
	if calendar == nil {
		return fmt.Errorf("Empty calendar cannot be saved")
	}

	subscriptionConfigJson, err := json.Marshal(calendar.Config)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		UPDATE calendars
		SET title = ?, updated_at = ?, subscription_config = ?
		WHERE id = ?`,
		calendar.Title, time.Now(), string(subscriptionConfigJson), calendar.Id)

	return err
}

func (r *Repo) SetCalendarRetrievedAt(calendarId string) error {
	_, err := r.db.Exec(`
		UPDATE calendars
		SET retrieved_at = ?
		WHERE id = ?`,
		time.Now(), calendarId)

	return err
}

func (r *Repo) DeleteCalendar(id string) error {
	_, err := r.db.Exec("DELETE FROM calendars WHERE id = ?", id)
	return err
}

func (r *Repo) GetSubscriptions(calendar *model.Calendar) ([]*model.Subscription, error) {
	rows, err := r.db.Query(`
        SELECT s.status, s.created_at, s.updated_at, t.id, t.title, t.updated_at, t.start_date, t.end_date, t.series, t.pdga_status, t.drating
        FROM subscriptions AS s
        LEFT JOIN tournaments AS t ON s.tournament_id = t.id
        WHERE s.calendar_id = ?
    `, calendar.Id)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*model.Subscription{}

	for rows.Next() {
		var seriesJson string
		t := model.Tournament{Series: []string{}}
		s := model.Subscription{Calendar: calendar, Tournament: &t}
		err := rows.Scan(&s.Status, &s.CreatedAt, &s.UpdatedAt, &t.Id, &t.Title, &t.UpdatedAt, &t.StartDate, &t.EndDate, &seriesJson, &t.PdgaTier, &t.DRating)

		if err != nil {
			return []*model.Subscription{}, err
		}

		if err := json.Unmarshal([]byte(seriesJson), &t.Series); err != nil {
			return []*model.Subscription{}, err
		}

		result = append(result, &s)
	}

	return result, nil
}

func (r *Repo) UpsertSubscription(subscription *model.Subscription) error {
	if subscription == nil || subscription.Calendar == nil || subscription.Tournament == nil {
		return fmt.Errorf("Empty subscription cannot be saved")
	}

	_, err := r.db.Exec(`
		INSERT INTO subscriptions (calendar_id, tournament_id, created_at, updated_at, status)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(calendar_id, tournament_id) DO UPDATE SET
		status=excluded.status,
        updated_at=excluded.updated_at`,
		subscription.Calendar.Id, subscription.Tournament.Id, time.Now(), time.Now(), subscription.Status)

	return err
}

func (r *Repo) CreateTurnamentHistory(tournament *model.Tournament) error {
	snapshot, err := json.Marshal(tournament)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO tournament_history (tournament_id, date, updated_at, snapshot)
		VALUES(?, ?, ?, ?)`,
		tournament.Id, time.Now(), tournament.UpdatedAt, snapshot)

	return err
}
