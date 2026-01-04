package gto

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/hashicorp/go-retryablehttp"
)

type EventDetails struct {
	ID                   int
	Series               string
	PDGAStatus           string
	DRatingConsideration bool
}

func FetchEventDetails(eventID int) (*EventDetails, error) {
	url := fmt.Sprintf("https://turniere.discgolf.de/index.php?p=events&sp=view&id=%d", eventID)
	log.Printf("Fetching %s", url)

	client := retryablehttp.NewClient()
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d for %s", resp.StatusCode, url)
	}

	return parseEventPage(resp.Body, eventID)
}

func parseEventPage(r io.Reader, eventID int) (*EventDetails, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	details := &EventDetails{
		ID: eventID,
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
			details.Series = value
		case "PDGA Status":
			details.PDGAStatus = value
		case "D-Rating Berücksichtigung":
			details.DRatingConsideration = (value == "Ja")
		}
	})

	return details, nil
}
