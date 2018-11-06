package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	fp "path/filepath"

	"sync"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/Financial-Times/go-logger"
)

const (
	personType    = "http://www.ft.com/ontology/person/Person"
	hasAuthor     = "http://www.ft.com/ontology/annotation/hasAuthor"
	TmeSource     = "tme"
	AuthorsSource = "authors"
	reqParamName  = "ids"
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
	Concordance    *ConcordanceService
	Suggesters     []Suggester
	BroaderExclude *BroaderExcludeService
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

type ConcordanceService struct {
	systemId            string
	name                string
	ConcordanceBaseURL  string
	ConcordanceEndpoint string
	Client              Client
	failureImpact       string
}

type FalconSuggester struct {
	SuggestionApi
}

type AuthorsSuggester struct {
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
	Flags []string
	Debug string
}

type SuggestionsResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type ConcordanceResponse struct {
	Concepts map[string]Concept `json:"concepts"`
}

func (sourceFlags *SourceFlags) hasFlag(value string) bool {
	for _, flag := range sourceFlags.Flags {
		if flag == value {
			return true
		}
	}
	return false
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

func NewAggregateSuggester(concordance *ConcordanceService, broaderExcludingService *BroaderExcludeService, suggesters ...Suggester) *AggregateSuggester {
	return &AggregateSuggester{
		Suggesters:     suggesters,
		Concordance:    concordance,
		BroaderExclude: broaderExcludingService,
	}
}

func (suggester *AggregateSuggester) GetSuggestions(payload []byte, tid string, flags SourceFlags) (SuggestionsResponse, error) {
	data, err := getXmlSuggestionRequestFromJson(payload)
	if flags.Debug != "" {
		log.WithTransactionID(tid).WithField("debug", flags.Debug).Info(string(data))
	}
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

	withoutDeprecated, err := suggester.filterByInternalConcordances(aggregateResp, tid, flags.Debug)
	if err != nil {
		return withoutDeprecated, err
	}

	results, err := suggester.BroaderExclude.excludeBroaderConceptsFromResponse(withoutDeprecated, tid, flags.Debug)
	if err != nil {
		log.WithError(err).Warn("Couldn't exclude broader concepts. Response might contain broader concepts as well")
	} else {
		withoutDeprecated = results
	}

	return withoutDeprecated, nil
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

func (suggester *AggregateSuggester) filterByInternalConcordances(s SuggestionsResponse, tid string, debugFlag string) (SuggestionsResponse, error) {
	if debugFlag != "" {
		log.WithTransactionID(tid).WithField("debug", debugFlag).Info("Calling internal concordances")
	}
	var filtered = SuggestionsResponse{Suggestions: make([]Suggestion, 0)}
	var concorded ConcordanceResponse
	if len(s.Suggestions) == 0 {
		log.WithTransactionID(tid).Info("No suggestions for calling internal concordances!")
		return filtered, nil
	}

	req, err := http.NewRequest("GET", suggester.Concordance.ConcordanceBaseURL+suggester.Concordance.ConcordanceEndpoint, nil)
	if err != nil {
		return filtered, err
	}

	queryParams := req.URL.Query()

	for _, suggestion := range s.Suggestions {
		queryParams.Add(reqParamName, fp.Base(suggestion.Concept.ID))
	}

	queryParams.Add("include_deprecated", "false")

	req.URL.RawQuery = queryParams.Encode()

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("X-Request-Id", tid)
	if debugFlag != "" {
		req.Header.Add("debug", debugFlag)
	}

	resp, err := suggester.Concordance.Client.Do(req)
	if err != nil {
		return filtered, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return filtered, err
	}

	err = json.Unmarshal(body, &concorded)
	if err != nil {
		return filtered, err
	}

	for id, c := range concorded.Concepts {
		for _, suggestion := range s.Suggestions {
			if id == fp.Base(suggestion.Concept.ID) {
				filtered.Suggestions = append(filtered.Suggestions, Suggestion{
					Predicate: suggestion.Predicate,
					Concept:   c,
				})
				break
			}
		}
	}
	return filtered, nil
}

func isNotAuthor(value Suggestion) bool {
	return !(value.Concept.Type == personType && value.Predicate == hasAuthor)
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

func (concordance *ConcordanceService) Check() health.Check {
	return health.Check{
		ID:               concordance.systemId,
		BusinessImpact:   concordance.failureImpact,
		Name:             fmt.Sprintf("%v Healthcheck", concordance.name),
		PanicGuide:       "https://dewey.in.ft.com/view/system/internal-concordances",
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
