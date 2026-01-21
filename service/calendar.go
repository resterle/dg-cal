package service

import (
	"crypto/rand"
	"math/big"

	"github.com/resterle/dg-cal/v2/model"
)

const idGroupLen = 4
const idGroups = 4
const charset = "abcdefghjkmnpqrstuvwxyz23456789"

type CalendarRepo interface {
	CreateCalendar(id, editId, title string, config model.SubscriptionConfig) error
	UpdateCalendar(calendar *model.Calendar) error
	GetCalendars() ([]*model.Calendar, error)
	GetCalendarById(id string) (*model.Calendar, error)
	GetCalendarByEditId(editId string) (*model.Calendar, error)
	GetCalendarUpdateCount() (map[int]int, error)
	SetCalendarRetrievedAt(calendarId string) error
	DeleteCalendar(id string) error
}

type CalId struct {
	t  int
	id string
}

func CalendarId(id string) CalId {
	return CalId{t: 0, id: id}
}

func CalendarEditId(id string) CalId {
	return CalId{t: 1, id: id}
}

type CalendarService struct {
	repo CalendarRepo
}

func NewCalendarService(repo CalendarRepo) *CalendarService {
	return &CalendarService{repo: repo}
}

func (s *CalendarService) CreateCalendar(title string, config model.SubscriptionConfig) (string, error) {
	id := rand.Text()
	editId := generateSecret()

	if err := s.repo.CreateCalendar(id, editId, title, config); err != nil {
		return "", err
	}

	return editId, nil
}

func (s *CalendarService) UpdateCalendar(calendar *model.Calendar) (*model.Calendar, error) {
	if err := s.repo.UpdateCalendar(calendar); err != nil {
		return nil, err
	}
	return calendar, nil
}

func (s *CalendarService) GetCalendar(id CalId) (*model.Calendar, error) {
	f := s.repo.GetCalendarById
	if id.t == 1 {
		f = s.repo.GetCalendarByEditId
	}
	return f(id.id)
}

func (s *CalendarService) GetUpdateCount() (map[int]int, error) {
	return s.repo.GetCalendarUpdateCount()
}

func (s *CalendarService) SetCalendarRetrievedAt(calendarId string) error {
	return s.repo.SetCalendarRetrievedAt(calendarId)
}

func (s *CalendarService) GetAllCalendars() ([]*model.Calendar, error) {
	return s.repo.GetCalendars()
}

func (s *CalendarService) DeleteCalendar(id string) error {
	return s.repo.DeleteCalendar(id)
}

func generateSecret() string {
	length := idGroupLen * idGroups

	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic(err)
		}
		result[i] = charset[num.Int64()]
	}

	return string(result)
}
