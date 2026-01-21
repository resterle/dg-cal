package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/resterle/dg-cal/v2/model"
	"github.com/resterle/dg-cal/v2/service"
)

const defaultLang = "de"
const langContextKey = "langContextKey"

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed common.css
var commonCSS []byte

//go:embed table-filters.js
var tableFiltersJS []byte

//go:embed fonts/*.woff2
var fontsFS embed.FS

type WebApp struct {
	calendaeService   CalendarServiceInterface
	tournamentService TournamentServiceInterface
	icsService        IcsServiceInterface
	templates         *template.Template
	translator        *Translator
	loc               *time.Location
	syncInterval      time.Duration
}

type CalendarServiceInterface interface {
	CreateCalendar(title string, config model.SubscriptionConfig) (string, error)
	GetCalendar(id service.CalId) (*model.Calendar, error)
	UpdateCalendar(calendar *model.Calendar) (*model.Calendar, error)
	GetUpdateCount() (map[int]int, error)
	GetAllCalendars() ([]*model.Calendar, error)
	DeleteCalendar(id string) error
}

type TournamentServiceInterface interface {
	GetTournaments() []*model.Tournament
	GetTournament(id int) *model.Tournament
	GetAllSeries(active ...bool) []string
	GetTournamentHistory(tournamentId int) ([]*model.Tournament, error)
	GetLastSync() *time.Time
}

type IcsServiceInterface interface {
	CreateIcs(id string) (string, error)
}

func NewWebApp(tournamentService TournamentServiceInterface, calendarService CalendarServiceInterface, icsService IcsServiceInterface, syncInterval time.Duration) WebApp {
	// Initialize translator with English as default language
	translator := NewTranslator(defaultLang)

	funcMap := template.FuncMap{
		// Translation functions
		"T": func(key string, lang string) string {
			return translator.T(lang, key)
		},
		"TArgs": func(key string, lang string, args ...interface{}) string {
			return translator.TWithArgs(lang, key, args...)
		},
		"contains": func(slice []int, item int) bool {
			return slices.Contains(slice, item)
		},
		"formatDate": func(t any) string {
			if date, ok := t.(time.Time); ok {
				return date.Format("2006-01-02")
			}
			return ""
		},
		"formatDay": func(t any) string {
			if date, ok := t.(time.Time); ok {
				return date.Format("2")
			}
			return ""
		},
		"formatMonth": func(t any) string {
			if date, ok := t.(time.Time); ok {
				monthKeys := map[time.Month]string{
					time.January: "month.jan", time.February: "month.feb", time.March: "month.mar",
					time.April: "month.apr", time.May: "month.may", time.June: "month.jun",
					time.July: "month.jul", time.August: "month.aug", time.September: "month.sep",
					time.October: "month.oct", time.November: "month.nov", time.December: "month.dec",
				}
				if key, ok := monthKeys[date.Month()]; ok {
					return translator.T("de", key)
				}
				return date.Format("Jan")
			}
			return ""
		},
		"formatYear": func(t any) string {
			if date, ok := t.(time.Time); ok {
				return date.Format("2006")
			}
			return ""
		},
		"formatWeekday": func(t any) string {
			if date, ok := t.(time.Time); ok {
				weekdayKeys := map[time.Weekday]string{
					time.Monday: "weekday.mon", time.Tuesday: "weekday.tue", time.Wednesday: "weekday.wed",
					time.Thursday: "weekday.thu", time.Friday: "weekday.fri", time.Saturday: "weekday.sat",
					time.Sunday: "weekday.sun",
				}
				if key, ok := weekdayKeys[date.Weekday()]; ok {
					return translator.T("de", key)
				}
				return date.Format("Mon")
			}
			return ""
		},
		"weekdayClass": func(t any) string {
			if date, ok := t.(time.Time); ok {
				weekday := date.Weekday()
				if weekday == time.Saturday {
					return "saturday"
				} else if weekday == time.Sunday {
					return "sunday"
				}
			}
			return ""
		},
		"sameDay": func(start, end any) bool {
			startDate, ok1 := start.(time.Time)
			endDate, ok2 := end.(time.Time)
			if ok1 && ok2 {
				return startDate.Year() == endDate.Year() &&
					startDate.Month() == endDate.Month() &&
					startDate.Day() == endDate.Day()
			}
			return false
		},
		"lower": func(s string) string {
			return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
		},
		"join": strings.Join,
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"formatAccessCode": func(code string) string {
			// Remove any existing dashes
			code = strings.ReplaceAll(code, "-", "")
			if len(code) != 16 {
				return code
			}
			// Format as xxxx-xxxx-xxxx-xxxx
			return code[0:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:16]
		},
	}

	loc, _ := time.LoadLocation("Europe/Berlin")
	templates := template.Must(template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html"))
	return WebApp{
		tournamentService: tournamentService,
		calendaeService:   calendarService,
		icsService:        icsService,
		templates:         templates,
		translator:        translator,
		loc:               loc,
		syncInterval:      syncInterval,
	}
}

type UpcomingRegistration struct {
	Lang          string
	Title         string
	OpensToday    bool
	OpensTomorrow bool
	OpensInDays   int
	StartTime     time.Time
}

type TournamentView struct {
	*model.Tournament
	HasActiveRegistration   bool
	ActiveRegistrationTitle string
	HasMultiplePhases       bool
	UpcomingRegistrations   []UpcomingRegistration
}

type TournamentYearGroup struct {
	Lang        string
	Year        int
	Tournaments []*TournamentView
}

type TournamentsPageData struct {
	Lang        string
	LastSync    string
	LastSyncISO string
	Groups      []TournamentYearGroup
}

func (app *WebApp) TournamentsHandler(w http.ResponseWriter, r *http.Request) {

	tournaments := app.tournamentService.GetTournaments()
	tournaments = slices.DeleteFunc(tournaments, func(t *model.Tournament) bool { return t.Status == model.TOURNAMENT_STATUS_CANCELLED })
	sort.Slice(tournaments, func(i, j int) bool {
		if tournaments[i].StartDate.Equal(tournaments[j].StartDate) {
			return tournaments[i].Id < tournaments[j].Id
		}
		return tournaments[i].StartDate.Before(tournaments[j].StartDate)
	})

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.Add(24 * time.Hour)
	dayAfterTomorrow := tomorrow.Add(24 * time.Hour)

	// Group tournaments by year
	yearGroups := make(map[int][]*TournamentView)
	years := []int{}

	for _, t := range tournaments {
		hasActiveReg := false
		activeRegTitle := ""
		var upcomingRegs []UpcomingRegistration
		hasMultiplePhases := len(t.Registrations) > 1

		for _, reg := range t.Registrations {
			// Check if this registration is currently active
			if now.After(reg.StartDate) && now.Before(reg.EndDate) {
				hasActiveReg = true
				activeRegTitle = reg.Title
			}
			// Check for upcoming registration (starts in the future, within 14 days)
			if reg.StartDate.After(now) {
				duration := reg.StartDate.Sub(now)
				days := int(duration.Hours() / 24)

				if days <= 14 {
					upcoming := UpcomingRegistration{
						Title:     reg.Title,
						StartTime: reg.StartDate,
					}
					if reg.StartDate.Before(tomorrow) {
						upcoming.OpensToday = true
					} else if reg.StartDate.Before(dayAfterTomorrow) {
						upcoming.OpensTomorrow = true
					} else {
						upcoming.OpensInDays = days
					}
					upcomingRegs = append(upcomingRegs, upcoming)
				}
			}
		}

		// Sort upcoming registrations by start time
		sort.Slice(upcomingRegs, func(i, j int) bool {
			return upcomingRegs[i].StartTime.Before(upcomingRegs[j].StartTime)
		})

		view := &TournamentView{
			Tournament:              t,
			HasActiveRegistration:   hasActiveReg,
			ActiveRegistrationTitle: activeRegTitle,
			HasMultiplePhases:       hasMultiplePhases,
			UpcomingRegistrations:   upcomingRegs,
		}

		year := t.StartDate.Year()
		if _, exists := yearGroups[year]; !exists {
			years = append(years, year)
		}
		yearGroups[year] = append(yearGroups[year], view)
	}

	// Sort years
	sort.Ints(years)

	// Build the final data structure
	groups := make([]TournamentYearGroup, 0, len(years))
	for _, year := range years {
		groups = append(groups, TournamentYearGroup{
			Year:        year,
			Tournaments: yearGroups[year],
		})
	}

	data := TournamentsPageData{
		Lang:        GetLanguageFromContext(r.Context()),
		Groups:      groups,
		LastSync:    app.lastSync(),
		LastSyncISO: app.lastSyncISO(),
	}

	app.addCachingHeader(w)
	if err := app.templates.ExecuteTemplate(w, "tournaments.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		app.removeCachingHeader(w)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type RegistrationWithTournament struct {
	TournamentId      int
	TournamentTitle   string
	TournamentDate    time.Time
	TournamentEndDate time.Time
	TournamentSeries  []string
	PhaseTitle        string
	RegistrationStart time.Time
	RegistrationEnd   time.Time
	IsActive          bool
	OpensToday        bool
	OpensTomorrow     bool
	ClosesToday       bool
	ClosesTomorrow    bool
	DaysLeft          int
	OpensInDays       int
}

type RegistrationsPageData struct {
	Lang        string
	Open        []RegistrationWithTournament
	Upcoming    []RegistrationWithTournament
	LastSync    string
	LastSyncISO string
}

func (app *WebApp) WelcomeHandler(w http.ResponseWriter, r *http.Request) {
	data := struct{ Lang string }{Lang: GetLanguageFromContext(r.Context())}
	if err := app.templates.ExecuteTemplate(w, "welcome.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) RegistrationsHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(app.loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.Add(24 * time.Hour)
	dayAfterTomorrow := tomorrow.Add(24 * time.Hour)

	openRegistrations := []RegistrationWithTournament{}
	upcomingRegistrations := []RegistrationWithTournament{}

	// Flatten all registrations from all tournaments
	for _, t := range app.tournamentService.GetTournaments() {
		for _, phase := range t.Registrations {
			// Skip closed registrations (registration end date has passed)
			if phase.EndDate.Before(now) {
				continue
			}

			// Check if registration is currently active
			isActive := now.After(phase.StartDate) && now.Before(phase.EndDate)

			// Check if registration opens today or tomorrow
			opensToday := !isActive && phase.StartDate.After(now) && phase.StartDate.Before(tomorrow)
			opensTomorrow := !isActive && phase.StartDate.After(tomorrow) && phase.StartDate.Before(dayAfterTomorrow)

			// Check if registration closes today or tomorrow
			closesToday := isActive && phase.EndDate.After(now) && phase.EndDate.Before(tomorrow)
			closesTomorrow := isActive && phase.EndDate.After(tomorrow) && phase.EndDate.Before(dayAfterTomorrow)

			// Calculate days left until registration closes
			daysLeft := 0
			if isActive {
				duration := phase.EndDate.Sub(now)
				daysLeft = int(duration.Hours() / 24)
				if daysLeft < 0 {
					daysLeft = 0
				}
			}

			// Calculate days until registration opens
			opensInDays := 0
			if !isActive && phase.StartDate.After(now) {
				duration := phase.StartDate.Sub(now)
				opensInDays = int(duration.Hours() / 24)
				if opensInDays < 0 {
					opensInDays = 0
				}
			}

			reg := RegistrationWithTournament{
				TournamentId:      t.Id,
				TournamentTitle:   t.Title,
				TournamentDate:    t.StartDate,
				TournamentEndDate: t.EndDate,
				TournamentSeries:  t.Series,
				PhaseTitle:        phase.Title,
				RegistrationStart: phase.StartDate,
				RegistrationEnd:   phase.EndDate,
				IsActive:          isActive,
				OpensToday:        opensToday,
				OpensTomorrow:     opensTomorrow,
				ClosesToday:       closesToday,
				ClosesTomorrow:    closesTomorrow,
				DaysLeft:          daysLeft,
				OpensInDays:       opensInDays,
			}

			if isActive {
				openRegistrations = append(openRegistrations, reg)
			} else {
				upcomingRegistrations = append(upcomingRegistrations, reg)
			}
		}
	}

	// Sort by registration start time (earliest first)
	sort.Slice(openRegistrations, func(i, j int) bool {
		return openRegistrations[i].RegistrationStart.Before(openRegistrations[j].RegistrationStart)
	})
	sort.Slice(upcomingRegistrations, func(i, j int) bool {
		return upcomingRegistrations[i].RegistrationStart.Before(upcomingRegistrations[j].RegistrationStart)
	})

	data := RegistrationsPageData{
		Lang:        GetLanguageFromContext(r.Context()),
		Open:        openRegistrations,
		Upcoming:    upcomingRegistrations,
		LastSync:    app.lastSync(),
		LastSyncISO: app.lastSyncISO(),
	}

	app.addCachingHeader(w)
	if err := app.templates.ExecuteTemplate(w, "registrations.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		app.removeCachingHeader(w)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) TournamentHandler(w http.ResponseWriter, r *http.Request) {
	tournaments := app.tournamentService.GetTournaments()
	w.Header().Set("Content-Type", "application/json")
	app.addCachingHeader(w)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(tournaments); err != nil {
		app.removeCachingHeader(w)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (app *WebApp) CreateCalendarFormHandler(w http.ResponseWriter, r *http.Request) {
	data := struct{ Lang string }{Lang: GetLanguageFromContext(r.Context())}
	if err := app.templates.ExecuteTemplate(w, "create-calendar.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) CreateCalendarHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	editId, err := app.calendaeService.CreateCalendar(title, model.SubscriptionConfig{Tournaments: []int{}, Series: []string{}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect to success page to prevent duplicate creation on refresh
	http.Redirect(w, r, "/calendar/created?id="+editId, http.StatusSeeOther)
}

func (app *WebApp) CalendarCreatedHandler(w http.ResponseWriter, r *http.Request) {
	editId := r.URL.Query().Get("id")
	if editId == "" {
		http.Error(w, "Calendar ID is required", http.StatusBadRequest)
		return
	}

	data := struct {
		Lang   string
		EditId string
	}{
		Lang:   GetLanguageFromContext(r.Context()),
		EditId: editId,
	}

	if err := app.templates.ExecuteTemplate(w, "calendar-created.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) IcsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	log.Printf("=> %+v", r.Header)

	result, err := app.icsService.CreateIcs(id)
	if err == service.NotFoundError {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"tournaments.ics\"")
	app.addCachingHeader(w)

	w.Write([]byte(result))
}

func (app *WebApp) EditCalendarFormHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Calendar ID is required", http.StatusBadRequest)
		return
	}

	calendar, err := app.calendaeService.GetCalendar(service.CalendarEditId(id))
	if err != nil {
		http.Error(w, "Calendar not found", http.StatusNotFound)
		return
	}

	if calendar == nil {
		http.Error(w, "Calendar not found", http.StatusNotFound)
		return
	}

	tournaments := slices.DeleteFunc(
		app.tournamentService.GetTournaments(),
		func(t *model.Tournament) bool { return t.Status == model.TOURNAMENT_STATUS_CANCELLED })
	sort.Slice(tournaments, func(i, j int) bool {
		if tournaments[i].StartDate.Equal(tournaments[j].StartDate) {
			return tournaments[i].Id < tournaments[j].Id
		}
		return tournaments[i].StartDate.Before(tournaments[j].StartDate)
	})

	series := app.tournamentService.GetAllSeries()

	scheme := "https"
	host := r.Host
	calendarUrl := fmt.Sprintf("%s://%s/ical/%s", scheme, host, calendar.Id)

	data := struct {
		Lang            string
		PageTitle       string
		FormAction      string
		CalendarUrl     string
		CalendarUrlInfo string
		BackLink        string
		ShowAdminLink   bool
		Calendar        *model.Calendar
		Tournaments     []*model.Tournament
		Series          []string
	}{
		Lang:            GetLanguageFromContext(r.Context()),
		PageTitle:       "Edit Calendar",
		FormAction:      "/calendar/edit/" + id,
		CalendarUrl:     calendarUrl,
		CalendarUrlInfo: "Use this link to subscribe to your calendar in your calendar app (Google Calendar, Apple Calendar, etc.)",
		BackLink:        "",
		ShowAdminLink:   false,
		Calendar:        calendar,
		Tournaments:     tournaments,
		Series:          series,
	}

	if err := app.templates.ExecuteTemplate(w, "calendar-form.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) EditCalendarHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Calendar ID is required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	calendar, err := app.calendaeService.GetCalendar(service.CalendarEditId(id))
	if err != nil || calendar == nil {
		http.Error(w, "Calendar not found", http.StatusNotFound)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Parse tournament IDs
	tournamentIds := []int{}
	if err := r.ParseForm(); err == nil {
		for _, idStr := range r.Form["tournaments"] {
			var id int
			if _, err := fmt.Sscanf(idStr, "%d", &id); err == nil {
				tournamentIds = append(tournamentIds, id)
			}
		}
	}

	// Parse series
	series := r.Form["series"]

	// Update calendar
	calendar.Title = title
	calendar.Config = &model.SubscriptionConfig{
		Tournaments: tournamentIds,
		Series:      series,
	}

	_, err = app.calendaeService.UpdateCalendar(calendar)
	if err != nil {
		http.Error(w, "Failed to update calendar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to edit page with success message
	http.Redirect(w, r, "/calendar/edit/"+id, http.StatusSeeOther)
}

func (app *WebApp) AccessCalendarFormHandler(w http.ResponseWriter, r *http.Request) {
	data := struct{ Lang string }{Lang: GetLanguageFromContext(r.Context())}
	if err := app.templates.ExecuteTemplate(w, "access-calendar.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) AccessCalendarHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	editId := r.FormValue("editId")
	if editId == "" {
		http.Error(w, "Edit ID is required", http.StatusBadRequest)
		return
	}

	// Redirect to edit page
	http.Redirect(w, r, "/calendar/edit/"+editId, http.StatusSeeOther)
}

func (app *WebApp) CommonCSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(commonCSS)
}

func (app *WebApp) TableFiltersJSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(tableFiltersJS)
}

func (app *WebApp) FontHandler(w http.ResponseWriter, r *http.Request) {
	fontName := r.PathValue("name")
	if fontName == "" {
		http.Error(w, "Font name is required", http.StatusBadRequest)
		return
	}

	fontPath := "fonts/" + fontName
	fontData, err := fontsFS.ReadFile(fontPath)
	if err != nil {
		http.Error(w, "Font not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(fontData)
}

func (app *WebApp) TournamentDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "Tournament ID is required", http.StatusBadRequest)
		return
	}

	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	tournament := app.tournamentService.GetTournament(id)
	if tournament == nil {
		http.Error(w, "Tournament not found", http.StatusNotFound)
		return
	}

	data := struct {
		Lang string
		*model.Tournament
	}{
		Lang:       GetLanguageFromContext(r.Context()),
		Tournament: tournament,
	}

	if err := app.templates.ExecuteTemplate(w, "tournament-detail.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) AdminHandler(w http.ResponseWriter, r *http.Request) {
	calendars, err := app.calendaeService.GetAllCalendars()
	if err != nil {
		log.Printf("Failed to get calendars: %v", err)
		http.Error(w, "Failed to retrieve calendars", http.StatusInternalServerError)
		return
	}

	// Sort calendars by creation date (newest first)
	sort.Slice(calendars, func(i, j int) bool {
		return calendars[i].CreatedAt.After(calendars[j].CreatedAt)
	})

	data := struct {
		Lang      string
		Calendars []*model.Calendar
	}{
		Lang:      GetLanguageFromContext(r.Context()),
		Calendars: calendars,
	}

	if err := app.templates.ExecuteTemplate(w, "admin.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) DeleteCalendarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	calendarId := r.PathValue("id")
	if calendarId == "" {
		http.Error(w, "Calendar ID is required", http.StatusBadRequest)
		return
	}

	err := app.calendaeService.DeleteCalendar(calendarId)
	if err != nil {
		log.Printf("Failed to delete calendar: %v", err)
		http.Error(w, "Failed to delete calendar", http.StatusInternalServerError)
		return
	}

	// Redirect back to admin page
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (app *WebApp) AdminViewCalendarHandler(w http.ResponseWriter, r *http.Request) {
	calendarId := r.PathValue("id")
	if calendarId == "" {
		http.Error(w, "Calendar ID is required", http.StatusBadRequest)
		return
	}

	calendar, err := app.calendaeService.GetCalendar(service.CalendarId(calendarId))
	if err != nil {
		http.Error(w, "Calendar not found", http.StatusNotFound)
		return
	}

	if calendar == nil {
		http.Error(w, "Calendar not found", http.StatusNotFound)
		return
	}

	// Format tournament IDs as comma-separated string
	tournamentIds := ""
	if calendar.Config != nil && len(calendar.Config.Tournaments) > 0 {
		ids := make([]string, len(calendar.Config.Tournaments))
		for i, id := range calendar.Config.Tournaments {
			ids[i] = fmt.Sprintf("%d", id)
		}
		tournamentIds = strings.Join(ids, ", ")
	}

	// Get all available series for dropdown
	series := app.tournamentService.GetAllSeries()

	data := struct {
		Lang          string
		Calendar      *model.Calendar
		TournamentIds string
		Series        []string
	}{
		Lang:          GetLanguageFromContext(r.Context()),
		Calendar:      calendar,
		TournamentIds: tournamentIds,
		Series:        series,
	}

	if err := app.templates.ExecuteTemplate(w, "admin-edit-calendar.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) AdminUpdateCalendarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	calendarId := r.PathValue("id")
	if calendarId == "" {
		http.Error(w, "Calendar ID is required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	calendar, err := app.calendaeService.GetCalendar(service.CalendarId(calendarId))
	if err != nil || calendar == nil {
		http.Error(w, "Calendar not found", http.StatusNotFound)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Parse tournament IDs from comma-separated string
	tournamentIds := []int{}
	tournamentIdsStr := strings.TrimSpace(r.FormValue("tournaments"))
	if tournamentIdsStr != "" {
		parts := strings.Split(tournamentIdsStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				var id int
				if _, err := fmt.Sscanf(part, "%d", &id); err == nil {
					tournamentIds = append(tournamentIds, id)
				}
			}
		}
	}

	// Parse series from form array
	series := r.Form["series"]

	// Update calendar
	calendar.Title = title
	calendar.Config = &model.SubscriptionConfig{
		Tournaments: tournamentIds,
		Series:      series,
	}

	_, err = app.calendaeService.UpdateCalendar(calendar)
	if err != nil {
		http.Error(w, "Failed to update calendar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to admin view page
	http.Redirect(w, r, "/admin/calendar/"+calendarId, http.StatusSeeOther)
}

type TournamentWithCount struct {
	Tournament *model.Tournament
	Count      int
}

func (app *WebApp) AdminTournamentsHandler(w http.ResponseWriter, r *http.Request) {
	tournaments := app.tournamentService.GetTournaments()

	// Get update counts for all tournaments
	updateCounts, err := app.calendaeService.GetUpdateCount()
	if err != nil {
		log.Printf("Failed to get update counts: %v", err)
		updateCounts = map[int]int{}
	}

	// Sort tournaments by last update (newest first), then by ID
	sort.Slice(tournaments, func(i, j int) bool {
		if tournaments[i].UpdatedAt.Equal(tournaments[j].UpdatedAt) {
			return tournaments[i].Id > tournaments[j].Id
		}
		return tournaments[i].UpdatedAt.After(tournaments[j].UpdatedAt)
	})

	// Create a slice to preserve the sorted order
	tournamentData := make([]TournamentWithCount, 0, len(tournaments))
	for _, t := range tournaments {
		count := updateCounts[t.Id]
		tournamentData = append(tournamentData, TournamentWithCount{
			Tournament: t,
			Count:      count,
		})
	}

	data := struct {
		Lang        string
		Tournaments []TournamentWithCount
	}{
		Lang:        GetLanguageFromContext(r.Context()),
		Tournaments: tournamentData,
	}

	if err := app.templates.ExecuteTemplate(w, "admin-tournaments.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) AdminTournamentHistoryHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "Tournament ID is required", http.StatusBadRequest)
		return
	}

	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	tournament := app.tournamentService.GetTournament(id)
	if tournament == nil {
		http.Error(w, "Tournament not found", http.StatusNotFound)
		return
	}

	history, err := app.tournamentService.GetTournamentHistory(id)
	if err != nil {
		log.Printf("Failed to get tournament history: %v", err)
		http.Error(w, "Failed to retrieve tournament history", http.StatusInternalServerError)
		return
	}

	// Sort history by UpdatedAt (newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].UpdatedAt.After(history[j].UpdatedAt)
	})

	data := struct {
		Lang       string
		Tournament *model.Tournament
		History    []*model.Tournament
	}{
		Lang:       GetLanguageFromContext(r.Context()),
		Tournament: tournament,
		History:    history,
	}

	if err := app.templates.ExecuteTemplate(w, "admin-tournament-history.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *WebApp) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Lang string
	}{
		Lang: GetLanguageFromContext(r.Context()),
	}
	w.WriteHeader(http.StatusNotFound)
	if err := app.templates.ExecuteTemplate(w, "404.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "404 - Page Not Found", http.StatusNotFound)
	}
}

// LoggingMiddleware logs every request
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			lang = defaultLang
		}

		ctx := context.WithValue(r.Context(), langContextKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetLanguageFromContext(ctx context.Context) string {
	lang, ok := ctx.Value(langContextKey).(string)
	if ok && lang != "" {
		return lang
	}
	return defaultLang
}

func (app *WebApp) lastSync() string {
	s := app.tournamentService.GetLastSync()
	lastSync := "never"
	if s != nil {
		lastSync = s.In(app.loc).Format("02.01.2006 15:04")
	}
	return lastSync
}

func (app *WebApp) lastSyncISO() string {
	s := app.tournamentService.GetLastSync()
	if s != nil {
		return s.UTC().Format(time.RFC3339)
	}
	return ""
}

func (app *WebApp) addCachingHeader(resp http.ResponseWriter) {
	nextSync := app.tournamentService.GetLastSync().Add(app.syncInterval)
	caheUntil := int((time.Until(nextSync) + 2*time.Minute).Seconds())

	resp.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", caheUntil))
}

func (app *WebApp) removeCachingHeader(resp http.ResponseWriter) {
	resp.Header().Del("Cache-Control")
}
