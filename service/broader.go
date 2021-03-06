package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	fp "path/filepath"
	"strings"

	"github.com/Financial-Times/go-fthealth/v1_1"
)

type BroaderConceptsProvider struct {
	systemID             string
	name                 string
	PublicThingsBaseURL  string
	PublicThingsEndpoint string
	Client               Client
	failureImpact        string
}

func NewBroaderConceptsProvider(publicThingsAPIBaseURL, publicThingsEndpoint string, client Client) *BroaderConceptsProvider {
	return &BroaderConceptsProvider{
		PublicThingsBaseURL:  publicThingsAPIBaseURL,
		PublicThingsEndpoint: publicThingsEndpoint,
		Client:               client,
		name:                 "public-things-api",
		systemID:             "public-things-api",
		failureImpact:        "Excluding broader concepts will not work",
	}
}

type broaderResponse struct {
	Things map[string]Thing `json:"things"`
}

type Thing struct {
	ID              string           `json:"id"`
	BroaderConcepts []BroaderConcept `json:"broaderConcepts"`
}

type BroaderConcept struct {
	ID string `json:"id"`
}

func (b *BroaderConceptsProvider) Check() v1_1.Check {
	return v1_1.Check{
		ID:               b.systemID,
		BusinessImpact:   b.failureImpact,
		Name:             fmt.Sprintf("%v Healthcheck", b.name),
		PanicGuide:       PanicGuideURL + b.systemID,
		Severity:         2,
		TechnicalSummary: fmt.Sprintf("%v is not available", b.name),
		Checker:          b.healthCheck,
	}
}

func (b *BroaderConceptsProvider) healthCheck() (string, error) {
	req, err := http.NewRequest("GET", b.PublicThingsBaseURL+"/__gtg", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")

	resp, err := b.Client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Health check returned a non-200 HTTP status: %v", resp.StatusCode)
	}
	return fmt.Sprintf("%v is healthy", b.name), nil
}

func (b *BroaderConceptsProvider) excludeBroaderConceptsFromResponse(suggestions map[int][]Suggestion, tid string) (map[int][]Suggestion, error) {
	var ids []string
	for _, sourceSuggestions := range suggestions {
		for _, suggestion := range sourceSuggestions {
			ids = append(ids, fp.Base(suggestion.ID))
		}
	}

	if len(ids) == 0 {
		return suggestions, nil
	}

	results := make(map[int][]Suggestion)
	broader, err := b.getBroaderConcepts(ids, tid)
	if err != nil {
		return suggestions, err
	}

	broaderConceptsChecker := make(map[string]bool)
	for _, thing := range broader.Things {
		for _, broaderConcept := range thing.BroaderConcepts {
			broaderConceptsChecker[fp.Base(broaderConcept.ID)] = true
		}
	}
	if len(broaderConceptsChecker) == 0 {
		return suggestions, nil
	}

	for mapIdx, sourceSuggestions := range suggestions {
		filteredSourceSuggestions := []Suggestion{}
		for _, suggestion := range sourceSuggestions {
			if broaderConceptsChecker[fp.Base(suggestion.ID)] {
				continue
			}
			filteredSourceSuggestions = append(filteredSourceSuggestions, suggestion)
		}
		results[mapIdx] = filteredSourceSuggestions
	}

	return results, nil
}

func (b *BroaderConceptsProvider) getBroaderConcepts(ids []string, tid string) (*broaderResponse, error) {
	var result broaderResponse
	preparedURL := fmt.Sprintf("%s/%s", strings.TrimRight(b.PublicThingsBaseURL, "/"), strings.Trim(b.PublicThingsEndpoint, "/"))
	req, err := http.NewRequest("GET", preparedURL, nil)
	if err != nil {
		return nil, err
	}

	queryParams := req.URL.Query()

	for _, id := range ids {
		queryParams.Add("uuid", id)
	}

	queryParams.Add("showRelationship", "broader")
	queryParams.Add("showRelationship", "broaderTransitive")

	req.URL.RawQuery = queryParams.Encode()

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("X-Request-Id", tid)

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non 200 status code returned: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
