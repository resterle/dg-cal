package service

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	ics "github.com/arran4/golang-ical"
)

type IcsService struct {
	calendarService   *CalendarService
	tournamentService *TournamentService
}

var NotFoundError error

func NewIcsService(calendarService *CalendarService, tournamentService *TournamentService) *IcsService {
	NotFoundError = errors.New("Not found")

	return &IcsService{calendarService: calendarService, tournamentService: tournamentService}
}

func (s *IcsService) CreateIcs(id string) (string, error) {
	calendar, err := s.calendarService.GetCalendar(CalendarId(id))
	if err != nil {
		return "", err
	}
	if calendar == nil {
		return "", NotFoundError
	}

	updateCount, err := s.calendarService.GetUpdateCount()
	if err != nil {
		return "", err
	}

	tournaments := s.tournamentService.GetTournamentsForSeries(calendar.Config.Series)
	for _, tid := range calendar.Config.Tournaments {
		tournament := s.tournamentService.GetTournament(tid)
		if slices.Contains(tournaments, tournament) {
			continue
		}
		tournaments = append(tournaments, tournament)
	}

	icsCal := ics.NewCalendar()
	icsCal.SetProductId("dg-cal v0.1")
	icsCal.SetMethod(ics.MethodPublish)
	icsCal.SetName(calendar.Title)
	for _, tournament := range tournaments {
		if tournament == nil {
			continue
		}
		e := icsCal.AddEvent(fmt.Sprintf("tournament-%d@dg-cal", tournament.Id))
		e.SetSequence(updateCount[tournament.Id])
		e.SetDtStampTime(tournament.UpdatedAt)
		e.SetSummary(tournament.Title)
		e.SetDescription(fmt.Sprintf("https://turniere.discgolf.de/index.php?p=events&sp=view&id=%d", tournament.Id))

		e.SetAllDayStartAt(tournament.StartDate)
		e.SetAllDayEndAt(tournament.EndDate.Add(time.Hour * 24))
		e.SetTimeTransparency(ics.TransparencyTransparent)

		e.AddProperty(ics.ComponentPropertyLocation, tournament.Localtion)
		/* Outlook issue
		geo := strings.Split(tournament.GeoLocation, ",")
		if len(geo) == 2 {
			e.SetGeo(geo[0], geo[1])
		}
		*/
		e.SetProperty("X-MICROSOFT-CDO-ALLDAYEVENT", "TRUE")
		for i, reg := range tournament.Registrations {
			re := icsCal.AddEvent(fmt.Sprintf("registration-%d-%d@dg-cal", tournament.Id, i))
			re.SetDtStampTime(tournament.UpdatedAt)
			re.SetSequence(updateCount[tournament.Id])
			re.SetSummary("Anmeldung: " + tournament.Title)
			re.SetDescription(fmt.Sprintf("%s\nhttps://turniere.discgolf.de/index.php?p=events&sp=view&id=%d", reg.Title, tournament.Id))
			re.SetStartAt(reg.StartDate)
			re.SetEndAt(reg.StartDate.Add(time.Hour * 2))
			re.AddProperty(ics.ComponentPropertyRelatedTo, e.Id())
			re.SetTimeTransparency(ics.TransparencyTransparent)

			a := re.AddAlarm()
			a.SetDescription(fmt.Sprintf("Anmeldung: %s (%s)", tournament.Title, reg.Title))
			a.SetAction(ics.ActionDisplay)
			a.SetTrigger("-PT15M")
		}
	}
	if err := s.calendarService.SetCalendarRetrievedAt(calendar.Id); err != nil {
		log.Printf("Error setting calender retieved at: %s", err.Error())
	}
	return icsCal.Serialize(), nil
}
