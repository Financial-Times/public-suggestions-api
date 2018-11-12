package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	health "github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/Financial-Times/go-logger"
	"io/ioutil"
	"net/http"
)

const (
	ontologyPersonType       = "http://www.ft.com/ontology/person/Person"
	ontologyLocationType     = "http://www.ft.com/ontology/Location"
	ontologyOrganisationType = "http://www.ft.com/ontology/organisation/Organisation"

	ontologyPublicCompanyType  = "http://www.ft.com/ontology/company/PublicCompany"
	ontologyPrivateCompanyType = "http://www.ft.com/ontology/company/PrivateCompany"
	ontologyCompanyType        = "http://www.ft.com/ontology/company/Company"

	ontologyTopicType = "http://www.ft.com/ontology/Topic"

	predicateHasAuthor = "http://www.ft.com/ontology/annotation/hasAuthor"

	TmeSource     = "tme"
	AuthorsSource = "authors"
	CesSource     = "ces"
)

var (
	NoContentError  = errors.New("Suggestion API returned HTTP 204")
	BadRequestError = errors.New("Suggestion API returned HTTP 400")

	PseudoConceptTypeAuthor = "author"

	PersonSourceParam       = "personSource"
	LocationSourceParam     = "locationSource"
	OrganisationSourceParam = "organisationSource"
	TopicSourceParam        = "topicSource"
	TypeSourceParams        = []string{PersonSourceParam, OrganisationSourceParam, LocationSourceParam, TopicSourceParam, PseudoConceptTypeAuthor}

	typeValidators = map[string]func(Suggestion) bool{
		PersonSourceParam: func(value Suggestion) bool {
			return value.Type == ontologyPersonType && value.Predicate != predicateHasAuthor
		},
		LocationSourceParam: func(value Suggestion) bool {
			return value.Type == ontologyLocationType

		},
		OrganisationSourceParam: func(value Suggestion) bool {
			return value.Type == ontologyOrganisationType ||
				value.Type == ontologyPublicCompanyType ||
				value.Type == ontologyPrivateCompanyType ||
				value.Type == ontologyCompanyType
		},
		TopicSourceParam: func(value Suggestion) bool {
			return value.Type == ontologyTopicType

		},
		PseudoConceptTypeAuthor: func(value Suggestion) bool {
			return value.Type == ontologyPersonType && value.Predicate == predicateHasAuthor
		},
	}
)

type JsonInput struct {
	Byline   string `json:"byline,omitempty"`
	Body     string `json:"bodyXML"`
	Headline string `json:"title,omitempty"`
}

type Client interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type Suggester interface {
	GetSuggestions(payload []byte, tid string, flags SourceFlags) (SuggestionsResponse, error)
	FilterSuggestions(suggestions []Suggestion, flags SourceFlags) []Suggestion
	GetName() string
}

type SuggestionApi struct {
	name                 string
	sourceName           string
	targetedConceptTypes []string
	apiBaseURL           string
	suggestionEndpoint   string
	client               Client
	systemId             string
	failureImpact        string
}

type FalconSuggester struct {
	SuggestionApi
}

type AuthorsSuggester struct {
	SuggestionApi
}

type OntotextSuggester struct {
	SuggestionApi
}

type Suggestion struct {
	Concept
	Predicate string `json:"predicate,omitempty"`
}

type Concept struct {
	ID         string `json:"id"`
	APIURL     string `json:"apiUrl,omitempty"`
	Type       string `json:"type,omitempty"`
	PrefLabel  string `json:"prefLabel,omitempty"`
	IsFTAuthor bool   `json:"isFTAuthor,omitempty"`
}

type SourceFlags struct {
	Flags map[string]string
	Debug string
}

type SuggestionsResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

func (sourceFlags *SourceFlags) hasFlag(value string, forConceptTypes []string) bool {
	for conceptType, source := range sourceFlags.Flags {
		// disambiguate between two suggesters with the same sourceName but with different targeted concept types
		if len(forConceptTypes) > 0 && !valueInSlice(conceptType, forConceptTypes) {
			continue
		}
		if source == value {
			return true
		}
	}
	return false
}

func NewFalconSuggester(falconSuggestionApiBaseURL, falconSuggestionEndpoint string, client Client) *FalconSuggester {
	return &FalconSuggester{SuggestionApi{
		apiBaseURL:           falconSuggestionApiBaseURL,
		suggestionEndpoint:   falconSuggestionEndpoint,
		client:               client,
		name:                 "Falcon Suggestion API",
		sourceName:           TmeSource,
		targetedConceptTypes: []string{LocationSourceParam, OrganisationSourceParam, PersonSourceParam, TopicSourceParam},
		systemId:             "falcon-suggestion-api",
		failureImpact:        "Suggestions from TME won't work",
	}}
}

func NewAuthorsSuggester(authorsSuggestionApiBaseURL, authorsSuggestionEndpoint string, client Client) *AuthorsSuggester {
	return &AuthorsSuggester{SuggestionApi{
		apiBaseURL:           authorsSuggestionApiBaseURL,
		suggestionEndpoint:   authorsSuggestionEndpoint,
		client:               client,
		name:                 "Authors Suggestion API",
		sourceName:           AuthorsSource,
		targetedConceptTypes: []string{PseudoConceptTypeAuthor},
		systemId:             "authors-suggestion-api",
		failureImpact:        "Suggesting authors from Concept Search won't work",
	}}
}

func NewOntotextSuggester(ontotextSuggestionApiBaseURL, ontotextSuggestionEndpoint string, client Client) *OntotextSuggester {
	return &OntotextSuggester{SuggestionApi{
		apiBaseURL:           ontotextSuggestionApiBaseURL,
		suggestionEndpoint:   ontotextSuggestionEndpoint,
		client:               client,
		name:                 "Ontotext Suggestion API",
		sourceName:           CesSource,
		targetedConceptTypes: []string{LocationSourceParam, OrganisationSourceParam, PersonSourceParam, TopicSourceParam},
		systemId:             "ontotext-suggestion-api",
		failureImpact:        "Suggesting locations, organisations and people from Ontotext won't work",
	}}
}

func (suggester *SuggestionApi) GetSuggestions(payload []byte, tid string, flags SourceFlags) (SuggestionsResponse, error) {
	if flags.Debug != "" {
		log.WithTransactionID(tid).WithField("Flags", flags.Flags).Infof("%s called", suggester.GetName())
	}
	if !flags.hasFlag(suggester.sourceName, suggester.targetedConceptTypes) {
		if flags.Debug != "" {
			log.WithTransactionID(tid).WithField("Flags", flags.Flags).Infof("%s skipped because of the flags", suggester.GetName())
		}
		return SuggestionsResponse{make([]Suggestion, 0)}, nil
	}

	req, err := http.NewRequest("POST", suggester.apiBaseURL+suggester.suggestionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return SuggestionsResponse{}, err
	}
	if flags.Debug != "" {
		req.Header.Add("debug", flags.Debug)
	}
	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Request-Id", tid)

	resp, err := suggester.client.Do(req)
	if err != nil {
		return SuggestionsResponse{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return SuggestionsResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return SuggestionsResponse{make([]Suggestion, 0)}, NoContentError
		}
		if resp.StatusCode == http.StatusBadRequest {
			return SuggestionsResponse{make([]Suggestion, 0)}, BadRequestError
		}
		return SuggestionsResponse{}, fmt.Errorf("%v returned HTTP %v", suggester.name, resp.StatusCode)
	}

	var response SuggestionsResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return SuggestionsResponse{}, err
	}
	return response, nil
}

func (suggester *SuggestionApi) FilterSuggestions(suggestions []Suggestion, flags SourceFlags) []Suggestion {
	filtered := []Suggestion{}

	for _, suggestion := range suggestions {
		for _, conceptType := range suggester.targetedConceptTypes {
			if flags.Flags[conceptType] == suggester.sourceName && typeValidators[conceptType](suggestion) {
				filtered = append(filtered, suggestion)
				break
			}
		}
	}

	return filtered
}

func (suggester *SuggestionApi) GetName() string {
	return suggester.name
}

func (suggester *SuggestionApi) Check() health.Check {
	return health.Check{
		ID:               suggester.systemId,
		BusinessImpact:   suggester.failureImpact,
		Name:             fmt.Sprintf("%v Healthcheck", suggester.name),
		PanicGuide:       "https://dewey.in.ft.com/view/system/public-suggestions-api",
		Severity:         2,
		TechnicalSummary: fmt.Sprintf("%v is not available", suggester.name),
		Checker:          suggester.healthCheck,
	}
}

func (suggester *SuggestionApi) healthCheck() (string, error) {
	req, err := http.NewRequest("GET", suggester.apiBaseURL+"/__gtg", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")

	resp, err := suggester.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Health check returned a non-200 HTTP status: %v", resp.StatusCode)
	}
	return fmt.Sprintf("%v is healthy", suggester.name), nil
}

func valueInSlice(val string, slice []string) bool {
	for _, sliceVal := range slice {
		if sliceVal == val {
			return true
		}
	}
	return false
}
