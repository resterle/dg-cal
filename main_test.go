package main_test

import (
	"os"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/resterle/dg-cal/v2/db"
	"github.com/resterle/dg-cal/v2/gto"
	"github.com/resterle/dg-cal/v2/model"
	"github.com/stretchr/testify/assert"
)

const TEST_DB = "test_db.db"

func TestSubscription(t *testing.T) {
	repo, err := db.NewRepo(TEST_DB)
	assert.NoError(t, err)
	defer repo.Close()
	defer os.Remove(TEST_DB)

	tournament := model.Tournament{
		Id:        1,
		StartDate: time.Now(),
		EndDate:   time.Now().Add(time.Hour * 2),
		Title:     "Foo",
		Series:    []string{"A"},
		DRating:   true,
		Registrations: []*model.Registration{
			&model.Registration{Title: "1. A", StartDate: time.Now(), EndDate: time.Now().Add(time.Minute * 10)},
			&model.Registration{Title: "2. A", StartDate: time.Now(), EndDate: time.Now().Add(time.Minute * 10)},
		},
	}
	err = repo.UpsertTournament(&tournament)
	assert.NoError(t, err)

	cal := model.Calendar{Id: "foo", Title: "The foo", Email: "hans@example.com", Config: &model.SubscriptionConfig{Tournaments: []int{}, Series: []string{"A"}}}
	err = repo.CreateCalendar("", "", "", model.SubscriptionConfig{})
	assert.NoError(t, err)

	sub := model.Subscription{Calendar: &cal, Tournament: &tournament, Status: model.SUBSCRIPTION_STATUS_INVITED}
	repo.UpsertSubscription(&sub)

	got, err := repo.GetSubscriptions(&cal)
	assert.NoError(t, err)

	assert.Len(t, got, 1)
}

func TestCal(t *testing.T) {

	s := gto.NewGtoService("sessionid", "userdata")
	r, err := s.FetchTournaments()

	assert.Len(t, r, 101)

	cal := ics.NewCalendar()

	main := cal.AddEvent("simple-event-1")
	main.SetSummary("Summary")
	main.SetDescription("Description")
	main.SetLocation("Conference Room A")

	tt := time.Now().Add(time.Minute * 60)
	main.SetStartAt(tt)
	main.SetEndAt(tt.Add(time.Minute * 20))

	// Optionally set organizer and attendees
	main.SetOrganizer("mailto:raphael.esterle@gmail.com")
	main.AddAttendee("mailto:hansahanz@gmail.com")
	main.AddProperty(ics.ComponentPropertyCategories, "Category")

	register := cal.AddEvent("simple-event-1-reg")
	register.SetSummary("Reg summary")
	register.SetDescription("Reg description")
	register.SetStartAt(tt.Add(-(time.Minute * 40)))
	register.SetEndAt(tt.Add(-(time.Minute * 30)))

	register.AddProperty(ics.ComponentPropertyRelatedTo, main.Id())
	register.AddProperty(ics.ComponentPropertyCategories, "Category")

	a := main.AddAlarm()
	a.AddComment("Alarm A")
	a.SetAction("DISPLAY")
	a.SetTrigger("-PT5M")

	// Write to file
	f, err := os.Create("meeting-test.ics")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.WriteString(cal.Serialize())
}

/*
func TestCal2(t *testing.T) {

	repo, err := db.NewRepo("events.db")
	assert.NoError(t, err)
	defer repo.Close()

	repo.GetCalendarById("5WL6WK4Z7CYKIDRWN4CVMBNQAI")

	r, err := gto.FetchTournaments()
	gto.FetchEventDetails(2497)

	repo.GetCalendarUpdateCount()

	assert.NoError(t, err)
	assert.Len(t, r, 101)
}
*/

func Test3(t *testing.T) {
	g := gto.NewGtoService("aa", "bb")
	g.FetchEventDetails(2507)
}
