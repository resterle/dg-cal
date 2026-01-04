package model

import (
	"time"

	ics "github.com/arran4/golang-ical"
)

const EVENT_TYPE_TURNAMENT = 0
const EVENT_TYPE_REGISTRATION = 1

type Tournament struct {
	Id           int
	UpdatedAt    time.Time
	StartDate    time.Time
	EndDate      time.Time
	Title        string
	Serie        string
	PdgaStatus   string
	DRating      bool
	Ics          ics.VEvent
	Registration *Registration
}

type Registration struct {
	StartDate time.Time
	EndDate   time.Time
	UpdatedAt time.Time
	Ics       ics.VEvent
}
