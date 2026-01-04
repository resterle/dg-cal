package gto

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	ics "github.com/arran4/golang-ical"
	"github.com/resterle/dg-cal/v2/model"
)

func FetchTournaments() (map[int]*model.Tournament, error) {
	icsEvents, err := fetchIcs()
	if err != nil {
		return map[int]*model.Tournament{}, err
	}
	return parseIcsEvents(icsEvents)
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

		parser := parseTournament
		if t == model.EVENT_TYPE_REGISTRATION {
			parser = parseRegistration
		}

		if err := parser(event, tournament); err != nil {
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
	tournament.Ics = *event
	dtStart, err := event.GetStartAt()
	if err != nil {
		return err
	}
	tournament.StartDate = dtStart

	dtEnd, err := event.GetEndAt()
	if err != nil {
		return err
	}
	tournament.EndDate = dtEnd

	summaryProp := event.GetProperty(ics.ComponentPropertySummary)
	summary := ""
	if summaryProp != nil {
		summary = summaryProp.Value
	}
	tournament.Title = summary

	updatedAt, err := event.GetDtStampTime()
	if err != nil {
		fmt.Printf("EEEE %+v", err)
		return err
	}
	tournament.UpdatedAt = updatedAt
	return nil
}

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
	tournament.Registration = &registration
	return nil
}
