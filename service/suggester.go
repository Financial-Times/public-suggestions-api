package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/Financial-Times/go-logger"
	"sync"
)

const (
	personType    = "http://www.ft.com/ontology/person/Person"
	hasAuthor     = "http://www.ft.com/ontology/annotation/hasAuthor"
	TmeSource     = "tme"
	AuthorsSource = "authors"
)

var NoContentError = errors.New("Suggestion API returned HTTP 204")
var BadRequestError = errors.New("Suggestion API returned HTTP 400")

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
	GetName() string
}

type AggregateSuggester struct {
	Suggesters []Suggester
}

type SuggestionApi struct {
	name               string
	flag               string
	apiBaseURL         string
	suggestionEndpoint string
	client             Client
	systemId           string
	failureImpact      string
}

type FalconSuggester struct {
	SuggestionApi
}

type AuthorsSuggester struct {
	SuggestionApi
}

type SuggestionsResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	Predicate      string `json:"predicate,omitempty"`
	Id             string `json:"id,omitempty"`
	ApiUrl         string `json:"apiUrl,omitempty"`
	PrefLabel      string `json:"prefLabel,omitempty"`
	SuggestionType string `json:"type,omitempty"`
	IsFTAuthor     bool   `json:"isFTAuthor,omitempty"`
}

type SourceFlags struct {
	Flags []string
}

func NewFalconSuggester(falconSuggestionApiBaseURL, falconSuggestionEndpoint string, client Client) *FalconSuggester {
	return &FalconSuggester{SuggestionApi{
		apiBaseURL:         falconSuggestionApiBaseURL,
		suggestionEndpoint: falconSuggestionEndpoint,
		client:             client,
		name:               "Falcon Suggestion API",
		flag:               TmeSource,
		systemId:           "falcon-suggestion-api",
		failureImpact:      "Suggestions from TME won't work",
	}}
}

func NewAuthorsSuggester(authorsSuggestionApiBaseURL, authorsSuggestionEndpoint string, client Client) *AuthorsSuggester {
	return &AuthorsSuggester{SuggestionApi{
		apiBaseURL:         authorsSuggestionApiBaseURL,
		suggestionEndpoint: authorsSuggestionEndpoint,
		client:             client,
		name:               "Authors Suggestion API",
		flag:               AuthorsSource,
		systemId:           "authors-suggestion-api",
		failureImpact:      "Suggesting authors from Concept Search won't work",
	}}
}

func NewAggregateSuggester(suggesters ...Suggester) *AggregateSuggester {
	return &AggregateSuggester{suggesters}
}

func (suggester *AggregateSuggester) GetSuggestions(payload []byte, tid string, flags SourceFlags) SuggestionsResponse {
	data, err := getXmlSuggestionRequestFromJson(payload)
	if err != nil {
		data = payload
	}
	var aggregateResp = SuggestionsResponse{Suggestions: make([]Suggestion, 0)}

	var mutex = sync.Mutex{}
	var wg = sync.WaitGroup{}

	var responseMap = map[int][]Suggestion{}
	for key, suggesterDelegate := range suggester.Suggesters {
		wg.Add(1)
		go func(i int, delegate Suggester) {
			resp, err := delegate.GetSuggestions(data, tid, flags)
			if err != nil {
				if err == NoContentError || err == BadRequestError {
					log.WithTransactionID(tid).WithField("tid", tid).Warn(err.Error())
				} else {
					log.WithTransactionID(tid).WithField("tid", tid).WithError(err).Errorf("Error calling %v", delegate.GetName())
				}
			}
			mutex.Lock()
			responseMap[i] = resp.Suggestions
			mutex.Unlock()
			wg.Done()
		}(key, suggesterDelegate)
	}
	wg.Wait()
	// preserve results order
	for i := 0; i < len(suggester.Suggesters); i++ {
		aggregateResp.Suggestions = append(aggregateResp.Suggestions, responseMap[i]...)
	}

	return aggregateResp
}

func filterOutAuthors(resp SuggestionsResponse) []Suggestion {
	i := 0
	for _, value := range resp.Suggestions {
		if isNotAuthor(value) {
			//retain suggestion
			resp.Suggestions[i] = value
			i++
		}
	}
	return resp.Suggestions[:i]
}

func isNotAuthor(value Suggestion) bool {
	return !(value.SuggestionType == personType && value.Predicate == hasAuthor)
}

func (suggester *FalconSuggester) GetSuggestions(payload []byte, tid string, flags SourceFlags) (SuggestionsResponse, error) {
	suggestions, err := suggester.SuggestionApi.GetSuggestions(payload, tid, flags)
	if err != nil {
		return suggestions, err
	}
	if flags.hasFlag(AuthorsSource) {
		suggestions.Suggestions = filterOutAuthors(suggestions)
	}
	return suggestions, err
}

func (suggester *SuggestionApi) GetSuggestions(payload []byte, tid string, flags SourceFlags) (SuggestionsResponse, error) {
	if !flags.hasFlag(suggester.flag) {
		return SuggestionsResponse{make([]Suggestion, 0)}, nil
	}

	req, err := http.NewRequest("POST", suggester.apiBaseURL+suggester.suggestionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return SuggestionsResponse{}, err
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

func getXmlSuggestionRequestFromJson(jsonData []byte) ([]byte, error) {

	var jsonInput JsonInput

	err := json.Unmarshal(jsonData, &jsonInput)
	if err != nil {
		return nil, err
	}

	jsonInput.Byline = TransformText(jsonInput.Byline,
		HtmlEntityTransformer,
		TagsRemover,
		OuterSpaceTrimmer,
		DuplicateWhiteSpaceRemover,
	)
	jsonInput.Body = TransformText(jsonInput.Body,
		PullTagTransformer,
		WebPullTagTransformer,
		TableTagTransformer,
		PromoBoxTagTransformer,
		WebInlinePictureTagTransformer,
		HtmlEntityTransformer,
		TagsRemover,
		OuterSpaceTrimmer,
		DuplicateWhiteSpaceRemover,
	)
	jsonInput.Headline = TransformText(jsonInput.Headline,
		HtmlEntityTransformer,
		TagsRemover,
		OuterSpaceTrimmer,
		DuplicateWhiteSpaceRemover,
	)

	data, err := json.Marshal(jsonInput)
	if err != nil {
		return nil, err
	}

	return data, nil
}
