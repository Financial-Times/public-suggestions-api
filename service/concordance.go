package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Financial-Times/go-fthealth/v1_1"
)

const idsParamName = "ids"

type ConcordanceService struct {
	systemId            string
	name                string
	ConcordanceBaseURL  string
	ConcordanceEndpoint string
	Client              Client
	failureImpact       string
}

type ConcordanceResponse struct {
	Concepts map[string]Concept `json:"concepts"`
}

func NewConcordance(internalConcordancesApiBaseURL, internalConcordancesEndpoint string, client Client) *ConcordanceService {
	return &ConcordanceService{
		ConcordanceBaseURL:  internalConcordancesApiBaseURL,
		ConcordanceEndpoint: internalConcordancesEndpoint,
		Client:              client,
		name:                "internal-concordances",
		systemId:            "internal-concordances",
		failureImpact:       "Suggestions won't work",
	}
}

func (concordance *ConcordanceService) Check() v1_1.Check {
	return v1_1.Check{
		ID:               concordance.systemId,
		BusinessImpact:   concordance.failureImpact,
		Name:             fmt.Sprintf("%v Healthcheck", concordance.name),
		PanicGuide:       "https://runbooks.in.ft.com/internal-concordances",
		Severity:         2,
		TechnicalSummary: fmt.Sprintf("%v is not available", concordance.name),
		Checker:          concordance.healthCheck,
	}
}

func (concordance *ConcordanceService) healthCheck() (string, error) {
	req, err := http.NewRequest("GET", concordance.ConcordanceBaseURL+"/__gtg", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")

	resp, err := concordance.Client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Health check returned a non-200 HTTP status: %v", resp.StatusCode)
	}
	return fmt.Sprintf("%v is healthy", concordance.name), nil
}

func (concordance *ConcordanceService) getConcordances(ids []string, tid string, debugFlag string) (ConcordanceResponse, error) {
	var concorded ConcordanceResponse
	req, err := http.NewRequest("GET", concordance.ConcordanceBaseURL+concordance.ConcordanceEndpoint, nil)
	if err != nil {
		return concorded, err
	}

	queryParams := req.URL.Query()

	for _, id := range ids {
		queryParams.Add(idsParamName, id)
	}

	queryParams.Add("include_deprecated", "false")

	req.URL.RawQuery = queryParams.Encode()

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("X-Request-Id", tid)
	if debugFlag != "" {
		req.Header.Add("debug", debugFlag)
	}

	resp, err := concordance.Client.Do(req)
	if err != nil {
		return concorded, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return concorded, fmt.Errorf("non 200 status code returned: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return concorded, err
	}

	err = json.Unmarshal(body, &concorded)
	return concorded, err
}
