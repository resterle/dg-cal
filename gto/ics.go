package gto

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/resterle/dg-cal/v2/model"
)

func (s *GtoService) FetchTournaments() (map[int]*model.Tournament, error) {
	result := map[int]*model.Tournament{}

	updates, err := s.fetchTournamentUpdates()
	if err != nil {
		return map[int]*model.Tournament{}, err
	}

	icsEvents, err := fetchIcs()
	if err != nil {
		return map[int]*model.Tournament{}, err
	}
	tournaments, err := parseIcsEvents(icsEvents)
	if err != nil {
		return map[int]*model.Tournament{}, err
	}

	for id, update := range updates {
		tournament := tournaments[id]
		if tournament == nil {
			if slices.Contains([]string{model.TOURNAMENT_STATUS_CANCELLED, model.TOURNAMENT_STATUS_DONE}, update.status) {
				tournament = &model.Tournament{Id: id, Status: update.status}
			} else {
				continue
			}
		}
		tournament.UpdatedAt = update.updated
		tournament.Status = update.status
		result[id] = tournament
	}

	return result, nil
}

func fetchIcs() ([]*ics.VEvent, error) {
	/*
		file, err := os.Open("events.ics")
		if err != nil {
			log.Fatalf("Failed to open events.ics: %v", err)
		}
		defer file.Close()
	*/

	resp, err := http.Get("https://turniere.discgolf.de/media/icals/events.ics")
	if err != nil {
		log.Fatalf("Failed to get events.ics: %v", err)
		return []*ics.VEvent{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return []*ics.VEvent{}, fmt.Errorf("Expected status %d got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse the iCalendar file
	cal, err := ics.ParseCalendar(resp.Body)
	if err != nil {
		log.Fatalf("Failed to parse calendar: %v", err)
	}

	return cal.Events(), nil
}

func parseIcsEvents(events []*ics.VEvent) (map[int]*model.Tournament, error) {
	result := map[int]*model.Tournament{}

	for _, event := range events {
		t, id, err := typeAndId(event.Id())
		if err != nil {
			return map[int]*model.Tournament{}, err
		}
		tournament, exists := result[id]
		if !exists {
			tournament = &model.Tournament{Id: id}
			result[id] = tournament
		}

		if t == model.EVENT_TYPE_REGISTRATION {
			continue
		}

		if err := parseTournament(event, tournament); err != nil {
			return map[int]*model.Tournament{}, err
		}
	}

	return result, nil
}

func typeAndId(uid string) (int, int, error) {
	s, ok := strings.CutSuffix(uid, "@turniere.discgolf.de")
	if !ok {
		return 0, 0, fmt.Errorf("Unknown uid fromat")
	}
	parts := strings.SplitAfterN(s, "-", 3)
	t := model.EVENT_TYPE_TURNAMENT
	tid_str := parts[1]
	if parts[0] == "registration-" {
		t = model.EVENT_TYPE_REGISTRATION
		tid_str = parts[2]
	}
	tid, err := strconv.Atoi(tid_str)
	if err != nil {
		return 0, 0, fmt.Errorf("Id is not an int")
	}
	return t, tid, nil
}

func parseTournament(event *ics.VEvent, tournament *model.Tournament) error {
	dtStart, err := event.GetStartAt()
	if err != nil {
		return err
	}
	tournament.StartDate = dtStart

	dtEnd, err := event.GetEndAt()
	if err != nil {
		return err
	}
	// Convert exclusive DTEND to an inclusive end date while ignoring DST hour shifts.
	tournament.EndDate = time.Date(dtEnd.Year(), dtEnd.Month(), dtEnd.Day(), 0, 0, 0, 0, dtEnd.Location()).AddDate(0, 0, -1)

	summaryProp := event.GetProperty(ics.ComponentPropertySummary)
	summary := ""
	if summaryProp != nil {
		summary = summaryProp.Value
	}
	tournament.Title = summary

	return nil
}

/*
func parseRegistration(event *ics.VEvent, tournament *model.Tournament) error {
	registration := model.Registration{Ics: *event}

	dtStart, err := event.GetStartAt()
	if err != nil {
		return err
	}
	registration.StartDate = dtStart

	dtEnd, err := event.GetEndAt()
	if err != nil {
		return err
	}
	registration.EndDate = dtEnd

	updatedAt, err := event.GetDtStampTime()
	if err != nil {
		fmt.Printf("EEEE %+v", err)
		return err
	}
	registration.UpdatedAt = updatedAt
	tournament.Registrations = &registration
	return nil
}
*/
