package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Financial-Times/go-fthealth/v1_1"
)

type ConceptBlacklister interface {
	IsBlacklisted(uuid string, bl Blacklist) bool
	GetBlacklist(tid string) (Blacklist, error)
	Check() v1_1.Check
}

type Blacklister struct {
	baseUrl       string
	endpoint      string
	client        Client
	systemID      string
	name          string
	failureImpact string
}

type Blacklist struct {
	UUIDS []string `json:"uuids"`
}

func NewConceptBlacklister(baseUrl string, endpoint string, client Client) ConceptBlacklister {
	return &Blacklister{
		baseUrl:       baseUrl,
		endpoint:      endpoint,
		client:        client,
		systemID:      "concept-suggestions-blacklister",
		name:          "concept-suggestions-blacklister",
		failureImpact: "Suggestions vetoing will not work",
	}
}

func (b *Blacklister) IsBlacklisted(conceptId string, bl Blacklist) bool {
	for _, uuid := range bl.UUIDS {
		if strings.Contains(conceptId, uuid) {
			return true
		}
	}
	return false
}

func (b *Blacklister) GetBlacklist(tid string) (Blacklist, error) {
	req, err := http.NewRequest("GET", b.baseUrl+b.endpoint, nil)
	if err != nil {
		return Blacklist{}, err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Request-Id", tid)

	resp, err := b.client.Do(req)
	if err != nil {
		return Blacklist{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Blacklist{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Blacklist{}, fmt.Errorf("concept-suggestions-blacklister returned HTTP %v", resp.StatusCode)
	}

	var blacklist Blacklist
	err = json.Unmarshal(body, &blacklist)
	if err != nil {
		return Blacklist{}, err
	}
	return blacklist, nil
}

func (b *Blacklister) Check() v1_1.Check {
	return v1_1.Check{
		ID:               b.systemID,
		BusinessImpact:   b.failureImpact,
		Name:             fmt.Sprintf("%v Healthcheck", b.name),
		PanicGuide:       "https://runbooks.in.ft.com/concept-suggestions-blacklister",
		Severity:         2,
		TechnicalSummary: fmt.Sprintf("%v is not available", b.name),
		Checker:          b.healthCheck,
	}
}

func (b *Blacklister) healthCheck() (string, error) {
	req, err := http.NewRequest("GET", b.baseUrl+"/__gtg", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")

	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Health check returned a non-200 HTTP status: %v", resp.StatusCode)
	}
	return fmt.Sprintf("%v is healthy", b.name), nil
}
