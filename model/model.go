package model

import (
	"time"
)

const EVENT_TYPE_TURNAMENT = 0
const EVENT_TYPE_REGISTRATION = 1

const TOURNAMENT_STATUS_PROVISIONAL = "PROVISIONAL"
const TOURNAMENT_STATUS_ANNOUNCED = "ANNOUNCED"
const TOURNAMENT_STATUS_REGISTRATION = "REGISTRATION"
const TOURNAMENT_STATUS_IN_PROGRESS = "IN PROGESS"
const TOURNAMENT_STATUS_DONE = "DONE"
const TOURNAMENT_STATUS_CANCELLED = "CANCELLED"

type Tournament struct {
	Id            int
	Status        string
	UpdatedAt     time.Time
	StartDate     time.Time
	EndDate       time.Time
	Title         string
	Localtion     string
	GeoLocation   string
	Series        []string
	PdgaTier      string
	PdgaId        string
	DRating       bool
	Registrations []*Registration
}

type Registration struct {
	Title     string
	StartDate time.Time
	EndDate   time.Time
}

type Calendar struct {
	Id          string
	Title       string
	Email       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RetrievedAt *time.Time
	Config      *SubscriptionConfig
}

type SubscriptionConfig struct {
	Tournaments []int
	Series      []string
}

const SUBSCRIPTION_STATUS_INVITED = "INVITED"
const SUBSCRIPTION_STATUS_ACCEPTED = "ACCEPTED"
const SUBSCRIPTION_STATUS_DECLINED = "DECLINED"
const SUBSCRIPTION_STATUS_CANCELLED = "CANCELLED"

type Subscription struct {
	Calendar   *Calendar
	Tournament *Tournament
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type RegistrationPhase struct {
	Name      string
	StartDate time.Time
	EndDate   time.Time
}

type EventDetails struct {
	ID                   int
	Title                string
	StartDate            time.Time
	EndDate              time.Time
	Location             string
	GeoLocation          string
	Series               []string
	PDGATier             string
	PDGAId               string
	DRatingConsideration bool
	RegistrationPhases   []RegistrationPhase
}
