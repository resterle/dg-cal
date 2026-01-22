package service

import (
	"log"
	"slices"
	"time"

	"github.com/resterle/dg-cal/v2/model"
)

type TournamentRepo interface {
	UpsertTournament(tournament *model.Tournament) error
	GetAllTournaments() ([]model.Tournament, error)
	CreateTurnamentHistory(tournament *model.Tournament) error
	UpsertRegistration(tournamentId int, registration *model.Registration) error
	GetTournamentHistory(id int) ([]*model.Tournament, error)
}

type GtoService interface {
	FetchEventDetails(eventID int) (*model.EventDetails, error)
	FetchTournaments() (map[int]*model.Tournament, error)
}

type TournamentService struct {
	tournaments map[int]*model.Tournament
	gtoService  GtoService
	repo        TournamentRepo
	lastSync    *time.Time
}

func NewTournamentService(repo TournamentRepo, gtoService GtoService) (*TournamentService, error) {
	s := TournamentService{tournaments: map[int]*model.Tournament{}, repo: repo, gtoService: gtoService}
	err := s.init()
	return &s, err
}

func (s *TournamentService) init() error {
	log.Printf("Load tournaments from db")
	t, err := s.repo.GetAllTournaments()
	if err != nil {
		return err
	}
	for i := range t {
		tt := t[i]
		s.tournaments[tt.Id] = &tt
	}
	log.Printf("Loaded %d tournaments from db", len(t))
	return nil
}

func (s *TournamentService) GetTournament(id int) *model.Tournament {
	return s.tournaments[id]
}

func (s *TournamentService) GetTournaments() []*model.Tournament {
	return s.getTournaments(func(t *model.Tournament) bool { return true })
}

func (s *TournamentService) GetTournamentsForSeries(series []string) []*model.Tournament {
	return s.getTournaments(func(t *model.Tournament) bool {
		for _, se := range t.Series {
			if slices.Contains(series, se) {
				return true
			}
		}
		return false
	})
}

func (s *TournamentService) GetAllSeries(active ...bool) []string {
	if len(active) != 1 {
		active = []bool{true}
	}

	set := map[string]any{}
	result := []string{}
	for _, t := range s.tournaments {
		if active[0] && slices.Contains([]string{model.TOURNAMENT_STATUS_CANCELLED, model.TOURNAMENT_STATUS_DONE}, t.Status) {
			continue
		}
		for _, series := range t.Series {
			if _, ok := set[series]; ok {
				continue
			}
			result = append(result, series)
			set[series] = struct{}{}
		}
	}
	return result
}

func (s *TournamentService) GetTournamentHistory(id int) ([]*model.Tournament, error) {
	return s.repo.GetTournamentHistory(id)
}

func (s *TournamentService) Sync() error {
	log.Printf("Tournament sync start")
	gtoTournaments, err := s.gtoService.FetchTournaments()
	if err != nil {
		return err
	}

	for _, fetchedTournament := range gtoTournaments {
		storedTournament := s.tournaments[fetchedTournament.Id]
		if storedTournament == nil || storedTournament.UpdatedAt.Before(fetchedTournament.UpdatedAt) {
			log.Printf("Storing tournament %d last update %v", fetchedTournament.Id, fetchedTournament.UpdatedAt)
			details, err := s.gtoService.FetchEventDetails(fetchedTournament.Id)
			if err != nil {
				log.Printf("Error: loading %d failed", fetchedTournament.Id)
				return err
			}
			fetchedTournament.Title = details.Title
			fetchedTournament.Series = details.Series
			fetchedTournament.PdgaTier = details.PDGATier
			fetchedTournament.PdgaId = details.PDGAId
			fetchedTournament.DRating = details.DRatingConsideration
			fetchedTournament.Localtion = details.Location
			fetchedTournament.GeoLocation = details.GeoLocation
			fetchedTournament.StartDate = details.StartDate
			fetchedTournament.EndDate = details.EndDate

			s.tournaments[fetchedTournament.Id] = fetchedTournament
			s.repo.UpsertTournament(fetchedTournament)

			for _, p := range details.RegistrationPhases {
				r := model.Registration{Title: p.Name, StartDate: p.StartDate, EndDate: p.EndDate}
				s.repo.UpsertRegistration(fetchedTournament.Id, &r)
				fetchedTournament.Registrations = append(fetchedTournament.Registrations, &r)
			}

			if err := s.repo.CreateTurnamentHistory(fetchedTournament); err != nil {
				log.Printf("Could not write tournament history: %s", err.Error())
				return err
			}
		}
	}

	t := time.Now()
	s.lastSync = &t

	log.Printf("Tournament sync done")
	return nil
}

func (s *TournamentService) GetLastSync() *time.Time {
	if s.lastSync == nil {
		return nil
	}
	t := *s.lastSync
	return &t
}

func (s *TournamentService) getTournaments(filter func(*model.Tournament) bool) []*model.Tournament {
	result := []*model.Tournament{}
	for _, t := range s.tournaments {
		if filter(t) {
			result = append(result, t)
		}
	}
	return result
}
