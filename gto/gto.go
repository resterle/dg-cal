package gto

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/resterle/dg-cal/v2/model"
)

const dateTimeLayout = "02.01.2006 15:04"
const dateLayout = "02.01.2006"

type GtoService struct {
	sessionId string
	loginData string
}

func NewGtoService(sessionId, loginData string) GtoService {
	return GtoService{sessionId: sessionId, loginData: loginData}
}

func (s *GtoService) FetchEventDetails(eventID int) (*model.EventDetails, error) {
	url := fmt.Sprintf("https://turniere.discgolf.de/index.php?p=events&sp=view&id=%d", eventID)

	req, err := retryablehttp.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCookies(req.Request)
	req.Header.Set("User-Agent", "dg-cal/0.1")

	client := retryablehttp.NewClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d for %s", resp.StatusCode, url)
	}

	return parseEventPage(resp.Body, eventID)
}

func parseEventPage(r io.Reader, eventID int) (*model.EventDetails, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	titleEl := doc.Find("h2").First()
	titleEl.Children().Remove()
	title := strings.TrimSpace(titleEl.Text())
	details := &model.EventDetails{
		ID:     eventID,
		Title:  title,
		Series: []string{},
	}

	var basisdatenTable *goquery.Selection
	doc.Find("h4.card-title").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "Basisdaten" {
			basisdatenTable = s.Closest(".card").Find("table")
		}
	})

	if basisdatenTable == nil || basisdatenTable.Length() == 0 {
		return nil, fmt.Errorf("could not find Basisdaten table")
	}

	basisdatenTable.Find("tr").Each(func(i int, tr *goquery.Selection) {
		tds := tr.Find("td")
		if tds.Length() < 2 {
			return
		}

		label := strings.TrimSpace(tds.Eq(0).Text())
		value := strings.TrimSpace(tds.Eq(1).Text())

		switch label {
		case "Serien":
			spans := tds.Eq(1).Find("span")
			for _, s := range spans.EachIter() {
				details.Series = append(details.Series, strings.TrimSpace(s.Text()))
			}
		case "PDGA Status":
			if value == "" {
				break
			}
			switch []byte(strings.ToLower(value))[0] {
			case 'c':
				value = "C"
			case 'b':
				value = "B"
			case 'a':
				value = "A"
			}
			details.PDGATier = value
			if value != "" {
				link := tds.Eq(1).Find("a").Eq(0)
				if href, ok := link.Attr("href"); ok {
					details.PDGAId = href
					if pdgaId, ok := strings.CutPrefix(href, "https://www.pdga.com/tour/event/"); ok {
						details.PDGAId = pdgaId
					}
				}
			}
		case "D-Rating Berücksichtigung":
			details.DRatingConsideration = (value == "Ja")
		case "Ort":
			a := tds.Eq(1).Find("a")
			if a.Length() == 1 {
				details.Location = strings.TrimSpace(a.Eq(0).Text())
			} else {
				details.Location = value
			}
			if href, exists := a.Attr("href"); exists {
				googleUrl, err := url.Parse(href)

				if err != nil {
					log.Printf("Coud not parse googe url %s %s", err, googleUrl)
				}
				geoLocation := strings.TrimPrefix(googleUrl.Path, "/maps/place/")
				if geoLocation != googleUrl.Path && len(strings.Split(geoLocation, ",")) == 2 {
					details.GeoLocation = geoLocation
				}
			}
		case "Turnierbetrieb":
			startDate, endDate, err := parseDateRange(value, dateLayout)
			if err != nil || startDate == nil {
				break
			}
			details.StartDate = *startDate
			if endDate == nil {
				details.EndDate = *startDate
				break
			}
			details.EndDate = *endDate
		}
	})

	details.RegistrationPhases = parseRegistrationPhases(doc)
	return details, nil
}

func parseRegistrationPhases(doc *goquery.Document) []model.RegistrationPhase {
	phases := []model.RegistrationPhase{}

	var anmeldephasenCard *goquery.Selection
	doc.Find("h4.card-title").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "Anmeldephasen" {
			anmeldephasenCard = s.Closest(".card")
		}
	})

	if anmeldephasenCard == nil || anmeldephasenCard.Length() == 0 {
		return phases
	}

	anmeldephasenCard.Find(".card-header").Each(func(i int, header *goquery.Selection) {
		h5 := header.Find("h5").First()
		if h5 == nil || h5.Length() == 0 {
			return
		}

		title := strings.TrimSpace(h5.Text())

		dateElem := header.Find("small").First()
		if dateElem == nil || dateElem.Length() == 0 {
			return
		}

		dateText := strings.TrimSpace(dateElem.Text())
		startDate, endDate, err := parseDateRange(dateText, dateTimeLayout)
		if err != nil {
			log.Printf("Error parsing registration: %s", err.Error())
			return
		}
		if startDate == nil || endDate == nil {
			log.Print("Error parsing registration: start or enddate is nil")
			return
		}

		phases = append(phases, model.RegistrationPhase{
			Name:      title,
			StartDate: *startDate,
			EndDate:   *endDate,
		})
	})

	return phases
}

func parseDateRange(dateText, layout string) (*time.Time, *time.Time, error) {
	parts := strings.Split(dateText, "-")
	if len(parts) == 0 {
		return nil, nil, nil
	}

	loc, _ := time.LoadLocation("Europe/Berlin")

	startDate, err := time.ParseInLocation(layout, strings.TrimSpace(parts[0]), loc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse start date: %w", err)
	}

	var endDate *time.Time = nil
	if len(parts) == 2 {
		d, err := time.ParseInLocation(layout, strings.TrimSpace(parts[1]), loc)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse end date: %w", err)
		}
		endDate = &d
	}

	return &startDate, endDate, nil
}

type tournamentUpdate struct {
	updated time.Time
	status  string
}

func (s *GtoService) fetchTournamentUpdates() (map[int]*tournamentUpdate, error) {
	url := "https://turniere.discgolf.de/index.php?p=events"

	req, err := retryablehttp.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCookies(req.Request)
	req.Header.Set("User-Agent", "dg-cal/0.1")

	client := retryablehttp.NewClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d for %s", resp.StatusCode, url)
	}

	return parseTournamentUpdates(resp.Body)
}

func parseTournamentUpdates(r io.Reader) (map[int]*tournamentUpdate, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	updates := make(map[int]*tournamentUpdate)
	loc, _ := time.LoadLocation("Europe/Berlin")

	table := doc.Find("table#list_tournaments")
	if table.Length() == 0 {
		return nil, fmt.Errorf("could not find tournaments table")
	}

	lastUpdateColIndex := -1
	table.Find("thead tr th").Each(func(i int, th *goquery.Selection) {
		headerText := strings.TrimSpace(th.Text())
		if headerText == "Letzte Änderung" {
			lastUpdateColIndex = i
		}
	})

	if lastUpdateColIndex == -1 {
		return nil, fmt.Errorf("could not find 'Letzte Änderung' column in table header")
	}

	titleColIndex := -1
	table.Find("thead tr th").Each(func(i int, th *goquery.Selection) {
		headerText := strings.TrimSpace(th.Text())
		if headerText == "Turnier" {
			titleColIndex = i
		}
	})

	if titleColIndex == -1 {
		return nil, fmt.Errorf("could not find 'Turnier' column in table header")
	}

	table.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
		// Find the tournament link in the first column
		link := row.Find("td").Eq(titleColIndex).Find("a").First()
		href, exists := link.Attr("href")
		if !exists {
			fmt.Println("A")
			return
		}

		// Extract tournament ID from the href
		tournamentID, err := extractTournamentID(href)
		if err != nil {
			fmt.Println("B")
			return
		}

		// Get the last update time from the dynamically found column
		el := row.Find("td").Eq(lastUpdateColIndex)
		lastUpdateText := strings.TrimSpace(el.Text())
		if lastUpdateText == "" {
			fmt.Println("C")
			return
		}

		if updates[tournamentID] != nil {
			fmt.Println("D")
			return
		}

		// Parse the timestamp
		lastUpdate, err := time.ParseInLocation("02.01.2006 15:04", lastUpdateText, loc)
		if err != nil {
			fmt.Println("E")
			return
		}

		status := model.TOURNAMENT_STATUS_ANNOUNCED
		badge := strings.TrimSpace(row.Find("td").Eq(0).Find("span").Text())
		switch strings.ToLower(badge) {
		case "abgesagt":
			status = model.TOURNAMENT_STATUS_CANCELLED
		case "vorläufig":
			status = model.TOURNAMENT_STATUS_PROVISIONAL
		}

		updates[tournamentID] = &tournamentUpdate{updated: lastUpdate, status: status}
	})

	return updates, nil
}

func extractTournamentID(href string) (int, error) {
	u, err := url.Parse(href)
	if err != nil {
		return 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	idStr := u.Query().Get("id")
	if idStr == "" {
		return 0, fmt.Errorf("no id parameter found in URL")
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse tournament ID: %w", err)
	}

	return id, nil
}

func (s GtoService) setCookies(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: s.sessionId})
	req.AddCookie(&http.Cookie{Name: "user_login_data", Value: s.loginData})
}
