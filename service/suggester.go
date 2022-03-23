package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/public-suggestions-api/reqorigin"
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
)

var (
	NoContentError  = &SuggesterErr{msg: "Suggestion API returned HTTP 204"}
	BadRequestError = &SuggesterErr{msg: "Suggestion API returned HTTP 400"}

	PseudoConceptTypeAuthor = "author"

	PersonSourceParam       = "personSource"
	LocationSourceParam     = "locationSource"
	OrganisationSourceParam = "organisationSource"
	TopicSourceParam        = "topicSource"

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

// SuggesterErr is error type returned suggester GetSuggestions method
type SuggesterErr struct {
	msg string
	err error
}

func (s *SuggesterErr) Error() string {
	if s.err != nil && len(s.msg) != 0 {
		return fmt.Sprintf("%s: %v", s.msg, s.err)
	}
	if s.err != nil {
		return s.err.Error()
	}
	return s.msg
}

func (s *SuggesterErr) Unwrap() error {
	return s.err
}

type Client interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type Suggester interface {
	GetSuggestions(payload []byte, tid, origin string) (SuggestionsResponse, error)
	FilterSuggestions(suggestions []Suggestion) []Suggestion
	GetName() string
}

type SuggestionApi struct {
	name                 string
	targetedConceptTypes []string
	apiBaseURL           string
	suggestionEndpoint   string
	client               Client
	systemId             string
	failureImpact        string
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

type SuggestionsResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

func NewAuthorsSuggester(authorsSuggestionApiBaseURL, authorsSuggestionEndpoint string, client Client) *AuthorsSuggester {
	return &AuthorsSuggester{SuggestionApi{
		apiBaseURL:           authorsSuggestionApiBaseURL,
		suggestionEndpoint:   authorsSuggestionEndpoint,
		client:               client,
		name:                 "Authors Suggestion API",
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
		targetedConceptTypes: []string{LocationSourceParam, OrganisationSourceParam, PersonSourceParam, TopicSourceParam},
		systemId:             "ontotext-suggestion-api",
		failureImpact:        "Suggesting locations, organisations and people from Ontotext won't work",
	}}
}

func (suggester *SuggestionApi) GetSuggestions(payload []byte, tid, origin string) (SuggestionsResponse, error) {
	req, err := http.NewRequest("POST", suggester.apiBaseURL+suggester.suggestionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return SuggestionsResponse{}, &SuggesterErr{err: err}
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Request-Id", tid)
	reqorigin.SetHeader(req, origin)

	resp, err := suggester.client.Do(req)
	if err != nil {
		return SuggestionsResponse{}, &SuggesterErr{err: err}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return SuggestionsResponse{}, &SuggesterErr{err: err}
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return SuggestionsResponse{make([]Suggestion, 0)}, NoContentError
		}
		if resp.StatusCode == http.StatusBadRequest {
			return SuggestionsResponse{make([]Suggestion, 0)}, BadRequestError
		}
		return SuggestionsResponse{}, &SuggesterErr{msg: fmt.Sprintf("%v returned HTTP %v", suggester.name, resp.StatusCode)}
	}

	var response SuggestionsResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return SuggestionsResponse{}, &SuggesterErr{err: err}
	}
	return response, nil
}

func (suggester *SuggestionApi) FilterSuggestions(suggestions []Suggestion) []Suggestion {
	var filtered []Suggestion

	for _, suggestion := range suggestions {
		for _, conceptType := range suggester.targetedConceptTypes {
			if typeValidators[conceptType](suggestion) {
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
		PanicGuide:       PanicGuideURL + suggester.systemId,
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
