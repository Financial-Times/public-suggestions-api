package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/Financial-Times/go-fthealth/v1_1"
)

var ErrConceptNotAllowed = errors.New("concept is not allowed")

type CachedConceptFilter struct {
	baseUrl       string
	endpoint      string
	client        Client
	systemID      string
	name          string
	failureImpact string

	deniedUUIDs []string
	uuidsMx     *sync.RWMutex
	dirtyCache  bool
}

func NewCachedConceptFilter(baseUrl string, endpoint string, client Client) *CachedConceptFilter {
	return &CachedConceptFilter{
		baseUrl:       baseUrl,
		endpoint:      endpoint,
		client:        client,
		systemID:      "concept-suggestions-blacklister",
		name:          "concept-suggestions-blacklister",
		failureImpact: "Suggestions vetoing will not work",
		uuidsMx:       &sync.RWMutex{},
		dirtyCache:    true,
	}
}

func (f *CachedConceptFilter) IsConceptAllowed(ctx context.Context, tid string, conceptID string) error {
	if f.dirtyCache {
		f.RefreshCache(ctx, tid)
	}
	if !f.isAllowed(conceptID) {
		return ErrConceptNotAllowed
	}
	return nil
}

func (f *CachedConceptFilter) RefreshCache(ctx context.Context, tid string) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseUrl+f.endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Request-Id", tid)

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("concept-suggestions-blacklister returned HTTP %v", resp.StatusCode)
	}

	var blacklist struct {
		UUIDS []string `json:"uuids"`
	}
	err = json.Unmarshal(body, &blacklist)
	if err != nil {
		return err
	}
	f.setDeniedUUIDs(blacklist.UUIDS)
	f.dirtyCache = false
	return nil
}

func (f *CachedConceptFilter) Check() v1_1.Check {
	return v1_1.Check{
		ID:               f.systemID,
		BusinessImpact:   f.failureImpact,
		Name:             fmt.Sprintf("%v Healthcheck", f.name),
		PanicGuide:       PanicGuideURL + f.systemID,
		Severity:         2,
		TechnicalSummary: fmt.Sprintf("%v is not available", f.name),
		Checker:          f.healthCheck,
	}
}

func (f *CachedConceptFilter) healthCheck() (string, error) {
	req, err := http.NewRequest("GET", f.baseUrl+"/__gtg", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Health check returned a non-200 HTTP status: %v", resp.StatusCode)
	}
	return fmt.Sprintf("%v is healthy", f.name), nil
}

func (f *CachedConceptFilter) isAllowed(conceptId string) bool {
	f.uuidsMx.RLock()
	defer f.uuidsMx.RUnlock()
	for _, uuid := range f.deniedUUIDs {
		if strings.Contains(conceptId, uuid) {
			return false
		}
	}
	return true
}

func (f *CachedConceptFilter) setDeniedUUIDs(uuids []string) {
	f.uuidsMx.Lock()
	defer f.uuidsMx.Unlock()
	f.deniedUUIDs = uuids
}
