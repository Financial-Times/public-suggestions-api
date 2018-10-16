package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	log "github.com/Financial-Times/go-logger"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const sampleJSONResponse = `{
    "suggestions": [{
            "predicate": "http://www.ft.com/ontology/annotation/mentions",
            "id": "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
            "apiUrl": "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
            "prefLabel": "Donald Kaberuka",
            "type": "http://www.ft.com/ontology/person/Person",
            "isFTAuthor": false
        },
        {
            "predicate": "http://www.ft.com/ontology/annotation/hasAuthor",
            "id": "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
            "apiUrl": "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
            "prefLabel": "Lawrence Summers",
            "type": "http://www.ft.com/ontology/person/Person",
            "isFTAuthor": true
        }
    ]
}`

const sampleFalconResponse = `{
	"suggestions":[
	  {
		"predicate":"http://www.ft.com/ontology/annotation/hasAuthor",
		"id":"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc494",
		"apiUrl":"http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc494",
		"prefLabel":"Adam Samson",
		"type":"http://www.ft.com/ontology/person/Person"
	  },
	  {
		"id":"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
		"apiUrl":"http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
		"prefLabel":"London",
		"type":"http://www.ft.com/ontology/Location"
	  },
	  {
		"id":"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
		"apiUrl":"http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a30",
		"prefLabel":"Eric Platt",
		"type":"http://www.ft.com/ontology/person/Person"
	  },
	  {
		"id":"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
		"apiUrl":"http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
		"prefLabel":"Apple",
		"type":"http://www.ft.com/ontology/organisation/Organisation"
	  },
	  {
		"id":"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
		"apiUrl":"http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
		"prefLabel":"London Politics",
		"type":"http://www.ft.com/ontology/Topic"
	  }
	]
  }`

const sampleOntotextResponse = `{
	"suggestions":[
	  {
		"id":"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
		"apiUrl":"http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
		"prefLabel":"London",
		"type":"http://www.ft.com/ontology/Location"
	  },
	  {
		"id":"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a385",
		"apiUrl":"http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a385",
		"prefLabel":"Eric Platt",
		"type":"http://www.ft.com/ontology/person/Person"
	  },
	  {
		"id":"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
		"apiUrl":"http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
		"prefLabel":"Apple",
		"type":"http://www.ft.com/ontology/organisation/Organisation"
	  }
	]
  }`

type mockHttpClient struct {
	mock.Mock
}

type mockResponseBody struct {
	mock.Mock
}

type mockSuggestionApi struct {
	mock.Mock
}

func (m *mockSuggestionApi) GetSuggestions(payload []byte, tid string, flags SourceFlags) (SuggestionsResponse, error) {
	args := m.Called(payload, tid)
	return args.Get(0).(SuggestionsResponse), args.Error(1)
}

func (m *mockSuggestionApi) GetName() string {
	return "Mock Suggestion API"
}

type mockSuggestionApiServer struct {
	mock.Mock
}

type ClosingBuffer struct {
	*bytes.Buffer
}

func (cb *ClosingBuffer) Close() (err error) {
	// do nothing
	return
}

func (m *mockSuggestionApiServer) startMockServer(t *testing.T) *httptest.Server {
	router := mux.NewRouter()
	router.HandleFunc("/content/suggest", func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		assert.Equal(t, "UPP public-suggestions-api", ua)

		contentTypeHeader := r.Header.Get("Content-Type")
		acceptHeader := r.Header.Get("Accept")
		tid := r.Header.Get("X-Request-Id")

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.True(t, len(body) > 0)

		respStatus, respBody := m.UploadRequest(body, tid, contentTypeHeader, acceptHeader)
		w.WriteHeader(respStatus)
		if respBody != nil {
			w.Write(respBody.([]byte))
		}
	}).Methods(http.MethodPost)

	router.HandleFunc("/__gtg", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(m.GTG())
	}).Methods(http.MethodGet)

	return httptest.NewServer(router)
}

func (m *mockSuggestionApiServer) GTG() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockSuggestionApiServer) UploadRequest(body []byte, tid, contentTypeHeader, acceptHeader string) (int, interface{}) {
	args := m.Called(body, tid, contentTypeHeader, acceptHeader)
	return args.Int(0), args.Get(1)
}

func (c *mockHttpClient) Do(req *http.Request) (resp *http.Response, err error) {
	args := c.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func (b *mockResponseBody) Read(p []byte) (n int, err error) {
	args := b.Called(p)
	return args.Int(0), args.Error(1)
}

func (b *mockResponseBody) Close() error {
	args := b.Called()
	return args.Error(0)
}

func TestMain(m *testing.M) {
	log.InitDefaultLogger("test")
	os.Exit(m.Run())
}

func TestFalconSuggester_GetSuggestionsSuccessfullyWithoutAuthors(t *testing.T) {
	expect := assert.New(t)

	expectedSuggestions := []Suggestion{
		{
			Predicate: "http://www.ft.com/ontology/annotation/mentions",
			Concept: Concept{
				ID:         "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
				APIURL:     "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
				PrefLabel:  "Donald Kaberuka",
				Type:       "http://www.ft.com/ontology/person/Person",
				IsFTAuthor: false,
			},
		},
		{
			Predicate: "http://www.ft.com/ontology/annotation/hasAuthor",
			Concept: Concept{
				ID:         "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
				APIURL:     "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
				PrefLabel:  "Lawrence Summers",
				Type:       "http://www.ft.com/ontology/person/Person",
				IsFTAuthor: true,
			},
		},
	}

	body, err := json.Marshal(&expectedSuggestions)
	expect.NoError(err)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", body, "tid_test", "application/json", "application/json").Return(http.StatusOK, []byte(sampleJSONResponse))
	server := mockServer.startMockServer(t)

	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.Len(actualSuggestions, 1)

	expect.Contains(actualSuggestions, expectedSuggestions[0])
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_GetSuggestionsWithServiceUnavailable(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", []byte("{}"), "tid_test", "application/json", "application/json").Return(http.StatusServiceUnavailable, nil)
	server := mockServer.startMockServer(t)

	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Error(err)
	expect.Equal("Falcon Suggestion API returned HTTP 503", err.Error())
	expect.Nil(suggestionResp.Suggestions)

	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_GetSuggestionsErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)
	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewFalconSuggester(":/", "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("parse ://content/suggest: missing protocol scheme", err.Error())
}

func TestFalconSuggester_GetSuggestionsErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewFalconSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("Http Client err", err.Error())
	mockClient.AssertExpectations(t)
}

func TestFalconSuggester_GetSuggestionsErrorOnResponseBodyRead(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockBody := new(mockResponseBody)

	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: mockBody}, nil)
	mockBody.On("Read", mock.AnythingOfType("[]uint8")).Return(0, errors.New("Read error"))
	mockBody.On("Close").Return(nil)

	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewFalconSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("Read error", err.Error())
	mockClient.AssertExpectations(t)
	mockBody.AssertExpectations(t)
}

func TestFalconSuggester_GetSuggestionsErrorOnEmptyBodyResponse(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", []byte("{}"), "tid_test", "application/json", "application/json").Return(http.StatusOK, []byte{})
	server := mockServer.startMockServer(t)

	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Error(err)
	expect.Equal("unexpected end of JSON input", err.Error())
	expect.Nil(suggestionResp.Suggestions)

	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200)
	server := mockServer.startMockServer(t)

	suggester := NewFalconSuggester(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("falcon-suggestion-api", check.ID)
	expect.Equal("Suggestions from TME won't work", check.BusinessImpact)
	expect.Equal("Falcon Suggestion API Healthcheck", check.Name)
	expect.Equal("https://dewey.in.ft.com/view/system/public-suggestions-api", check.PanicGuide)
	expect.Equal("Falcon Suggestion API is not available", check.TechnicalSummary)
	expect.Equal(uint8(2), check.Severity)
	expect.NoError(err)
	expect.Equal("Falcon Suggestion API is healthy", checkResult)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_CheckHealthUnhealthy(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(503)
	server := mockServer.startMockServer(t)

	suggester := NewFalconSuggester(server.URL, "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	expect.Empty(checkResult)
	assert.Equal(t, "Health check returned a non-200 HTTP status: 503", err.Error())
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_CheckHealthErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	suggester := NewFalconSuggester(":/", "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "parse ://__gtg: missing protocol scheme", err.Error())
	expect.Empty(checkResult)
}

func TestFalconSuggester_CheckHealthErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewFalconSuggester("http://test-url", "/__gtg", mockClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "Http Client err", err.Error())
	expect.Empty(checkResult)
	mockClient.AssertExpectations(t)
}

func TestAuthorsSuggester_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200)
	server := mockServer.startMockServer(t)

	suggester := NewAuthorsSuggester(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("authors-suggestion-api", check.ID)
	expect.Equal("Suggesting authors from Concept Search won't work", check.BusinessImpact)
	expect.Equal("Authors Suggestion API Healthcheck", check.Name)
	expect.Equal("https://dewey.in.ft.com/view/system/public-suggestions-api", check.PanicGuide)
	expect.Equal("Authors Suggestion API is not available", check.TechnicalSummary)
	expect.Equal(uint8(2), check.Severity)
	expect.NoError(err)
	expect.Equal("Authors Suggestion API is healthy", checkResult)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestAuthorsSuggester_CheckHealthUnhealthy(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(503)
	server := mockServer.startMockServer(t)

	suggester := NewAuthorsSuggester(server.URL, "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	expect.Empty(checkResult)
	assert.Equal(t, "Health check returned a non-200 HTTP status: 503", err.Error())
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestAuthorsSuggester_CheckHealthErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	suggester := NewAuthorsSuggester(":/", "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "parse ://__gtg: missing protocol scheme", err.Error())
	expect.Empty(checkResult)
}

func TestAuthorsSuggester_CheckHealthErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewAuthorsSuggester("http://test-url", "/__gtg", mockClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "Http Client err", err.Error())
	expect.Empty(checkResult)
	mockClient.AssertExpectations(t)
}

func TestOntotextSuggester_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200)
	server := mockServer.startMockServer(t)

	suggester := NewOntotextSuggester(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("ontotext-suggestion-api", check.ID)
	expect.Equal("Suggesting locations, organisations and person from Ontotext won't work", check.BusinessImpact)
	expect.Equal("Ontotext Suggestion API Healthcheck", check.Name)
	expect.Equal("https://dewey.in.ft.com/view/system/public-suggestions-api", check.PanicGuide)
	expect.Equal("Ontotext Suggestion API is not available", check.TechnicalSummary)
	expect.Equal(uint8(2), check.Severity)
	expect.NoError(err)
	expect.Equal("Ontotext Suggestion API is healthy", checkResult)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestOntotextSuggester_CheckHealthUnhealthy(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(503)
	server := mockServer.startMockServer(t)

	suggester := NewOntotextSuggester(server.URL, "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	expect.Empty(checkResult)
	assert.Equal(t, "Health check returned a non-200 HTTP status: 503", err.Error())
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestOntotextSuggester_CheckHealthErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	suggester := NewOntotextSuggester(":/", "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "parse ://__gtg: missing protocol scheme", err.Error())
	expect.Empty(checkResult)
}

func TestOntotextSuggester_CheckHealthErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewOntotextSuggester("http://test-url", "/__gtg", mockClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "Http Client err", err.Error())
	expect.Empty(checkResult)
	mockClient.AssertExpectations(t)
}

func TestConcordanceService_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200).Once()
	server := mockServer.startMockServer(t)

	suggester := NewConcordance(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("internal-concordances", check.ID)
	expect.Equal("Suggestions won't work", check.BusinessImpact)
	expect.Equal("internal-concordances Healthcheck", check.Name)
	expect.Equal("https://dewey.in.ft.com/view/system/internal-concordances", check.PanicGuide)
	expect.Equal("internal-concordances is not available", check.TechnicalSummary)
	expect.Equal(uint8(2), check.Severity)
	expect.NoError(err)
	expect.Equal("internal-concordances is healthy", checkResult)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestConcordanceService_CheckHealthUnhealthy(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(503)
	server := mockServer.startMockServer(t)

	suggester := NewConcordance(server.URL, "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	expect.Empty(checkResult)
	assert.Equal(t, "Health check returned a non-200 HTTP status: 503", err.Error())
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestConcordanceService_CheckHealthErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	suggester := NewConcordance(":/", "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "parse ://__gtg: missing protocol scheme", err.Error())
	expect.Empty(checkResult)
}

func TestConcordanceService_CheckHealthErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewConcordance("http://test-url", "/__gtg", mockClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "Http Client err", err.Error())
	expect.Empty(checkResult)
	mockClient.AssertExpectations(t)
}

func TestAggregateSuggester_GetSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	suggestionApi := new(mockSuggestionApi)
	mockClient := new(mockHttpClient)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)

	falconSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		Suggestion{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "falcon-suggestion-api",
				APIURL:     "apiurl1",
				PrefLabel:  "prefLabel1",
				Type:       ontologyPersonType},
		},
	},
	}
	authorsSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		Suggestion{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       ontologyPersonType},
		},
	},
	}
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(falconSuggestion, nil).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["falcon-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "falcon-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: ontologyPersonType,
	}
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer, StatusCode: http.StatusOK}, nil)

	defaultConceptsSources := buildDefaultConceptSources()

	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, suggestionApi, suggestionApi)
	response, _ := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Len(response.Suggestions, 2)

	expect.Contains(response.Suggestions, falconSuggestion.Suggestions[0])
	expect.Contains(response.Suggestions, authorsSuggestion.Suggestions[0])

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetPersonSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)
	suggestionApi := new(mockSuggestionApi)
	falconSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "falcon-suggestion-api",
				APIURL:     "apiurl1",
				PrefLabel:  "prefLabel1",
				Type:       ontologyPersonType,
			},
		},
	}}
	authorsSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       ontologyPersonType,
			},
		},
	}}
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(falconSuggestion, nil).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()

	defaultConceptsSources := buildDefaultConceptSources()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["falcon-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "falcon-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: ontologyPersonType,
	}

	mockClient := new(mockHttpClient)
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer, StatusCode: http.StatusOK}, nil)

	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, suggestionApi, suggestionApi)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.NoError(err)
	expect.Len(response.Suggestions, 2)

	expect.Contains(response.Suggestions, falconSuggestion.Suggestions[0])
	expect.Contains(response.Suggestions, authorsSuggestion.Suggestions[0])

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetAuthorSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	// create falcon response mock
	falconMock := new(mockHttpClient)
	falconMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{
				"suggestions":[
					{
						"id":             "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392b5",
						"apiUrl":         "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392b5",
						"prefLabel":      "Lawrence Summers",
						"type": 		  "http://www.ft.com/ontology/person/Person"
					},
					{
						"predicate":      "http://www.ft.com/ontology/annotation/hasAuthor",
						"id":             "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e139265",
						"apiUrl":         "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e139265",
						"prefLabel":      "Lawrence Summers",
						"type":           "http://www.ft.com/ontology/person/Person",
						"isFTAuthor":     true
					}
				]
			}`)),
		StatusCode: http.StatusOK,
	}, nil)
	authorsMock := new(mockHttpClient)

	// authors response mock
	authorsMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{
				"suggestions":[
					{
						"predicate":      "http://www.ft.com/ontology/annotation/hasAuthor",
						"id":             "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e139260",
						"apiUrl":         "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e139260",
						"prefLabel":      "Lawrence Summers",
						"type":           "http://www.ft.com/ontology/person/Person",
						"isFTAuthor":		true
					}
				]
			}`)),
		StatusCode: http.StatusOK,
	}, nil)

	// mock internal concordances response
	mockInternalConcResp := ConcordanceResponse{
		Concepts: map[string]Concept{
			"9a5e3b4a-55da-498c-816f-9c534e1392b5": Concept{
				ID:        "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392b5",
				APIURL:    "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392b5",
				PrefLabel: "Lawrence Summers",
				Type:      "http://www.ft.com/ontology/person/Person",
			},
			"9a5e3b4a-55da-498c-816f-9c534e139260": Concept{
				ID:         "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e139260",
				APIURL:     "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e139260",
				PrefLabel:  "Lawrence Summers",
				Type:       "http://www.ft.com/ontology/person/Person",
				IsFTAuthor: true,
			},
		},
	}

	mockClient := new(mockHttpClient)
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}
	req, err := http.NewRequest("GET", "internalConcordancesHost/internalconcordances", nil)
	expect.NoError(err)

	queryParams := req.URL.Query()
	queryParams.Add(reqParamName, "9a5e3b4a-55da-498c-816f-9c534e1392b5")
	queryParams.Add(reqParamName, "9a5e3b4a-55da-498c-816f-9c534e139260")
	queryParams.Add("include_deprecated", "false")
	req.URL.RawQuery = queryParams.Encode()

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("X-Request-Id", "tid_test")
	mockClient.On("Do", req).Return(&http.Response{Body: buffer, StatusCode: http.StatusOK}, nil)

	// create all the services
	defaultConceptsSources := buildDefaultConceptSources()
	falconSuggester := NewFalconSuggester("falconUrl", "falconEndpoint", falconMock)
	authorsSuggester := NewAuthorsSuggester("authorsUrl", "authorsEndpoint", authorsMock)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, falconSuggester, authorsSuggester)

	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.NoError(err)
	expect.Len(response.Suggestions, 2)

	sort.Slice(response.Suggestions, func(i, j int) bool {
		return response.Suggestions[i].Concept.ID < response.Suggestions[j].Concept.ID
	})
	expect.Equal("http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e139260", response.Suggestions[0].ID)
	expect.Equal("http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392b5", response.Suggestions[1].ID)
}

func TestAggregateSuggester_GetEmptySuggestionsArrayIfNoAggregatedSuggestionAvailable(t *testing.T) {
	expect := assert.New(t)
	suggestionApi := new(mockSuggestionApi)
	mockConcordance := new(ConcordanceService)
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{}, errors.New("Falcon err"))

	defaultConceptsSources := buildDefaultConceptSources()
	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, suggestionApi, suggestionApi)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.NoError(err)
	expect.Len(response.Suggestions, 0)
	expect.NotNil(response.Suggestions)

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetSuggestionsNoErrorForFalconSuggestionApi(t *testing.T) {
	expect := assert.New(t)

	suggestionApi := new(mockSuggestionApi)
	mockClient := new(mockHttpClient)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: ontologyPersonType,
	}
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer, StatusCode: http.StatusOK}, nil)

	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{}, errors.New("Falcon err")).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{Suggestions: []Suggestion{
		Suggestion{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       ontologyPersonType,
			},
		},
	},
	}, nil).Once()

	defaultConceptsSources := buildDefaultConceptSources()
	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, suggestionApi, suggestionApi)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.NoError(err)
	expect.Len(response.Suggestions, 1)

	expect.Equal(response.Suggestions[0].Concept.ID, "authors-suggestion-api")

	suggestionApi.AssertExpectations(t)
}

func TestGetSuggestions_NoResultsForAuthorsTmeSourceFlag(t *testing.T) {
	expect := assert.New(t)
	suggester := AuthorsSuggester{}
	defaultConceptsSources := buildDefaultConceptSources()
	response, _ := suggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Equal(response.Suggestions, []Suggestion{})
}

func TestAuthorsSuggester_GetSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	expectedSuggestions := []Suggestion{
		{
			Predicate: "http://www.ft.com/ontology/annotation/mentions",
			Concept: Concept{
				ID:         "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
				APIURL:     "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
				PrefLabel:  "Donald Kaberuka",
				Type:       "http://www.ft.com/ontology/person/Person",
				IsFTAuthor: false,
			},
		},
		{
			Predicate: "http://www.ft.com/ontology/annotation/hasAuthor",
			Concept: Concept{
				ID:         "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
				APIURL:     "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
				PrefLabel:  "Lawrence Summers",
				Type:       "http://www.ft.com/ontology/person/Person",
				IsFTAuthor: true,
			},
		},
	}

	body, err := json.Marshal(&expectedSuggestions)
	expect.NoError(err)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", body, "tid_test", "application/json", "application/json").Return(http.StatusOK, []byte(sampleJSONResponse))
	server := mockServer.startMockServer(t)

	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewAuthorsSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.True(len(actualSuggestions) == len(expectedSuggestions))

	for _, expected := range expectedSuggestions {
		expect.Contains(actualSuggestions, expected)
	}
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestAggregateSuggester_GetSuggestionsSuccessfullyResponseFiltered(t *testing.T) {
	expect := assert.New(t)

	expectedSuggestions := []Suggestion{
		{
			Predicate: "http://www.ft.com/ontology/annotation/mentions",
			Concept: Concept{
				ID:         "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
				APIURL:     "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
				PrefLabel:  "Donald Kaberuka",
				Type:       "http://www.ft.com/ontology/person/Person",
				IsFTAuthor: false,
			},
		},
	}

	body, err := json.Marshal(&expectedSuggestions)
	expect.NoError(err)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", body, "tid_test", "application/json", "application/json").Return(http.StatusOK, []byte(sampleJSONResponse))
	server := mockServer.startMockServer(t)

	defaultConceptsSources := buildDefaultConceptSources()
	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.Equal(1, len(actualSuggestions))

	for _, expected := range expectedSuggestions {
		expect.Contains(actualSuggestions, expected)
	}
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestAggregateSuggester_GetSuggestionsSuccessfullyResponseFilteredCesVSTme(t *testing.T) {
	expect := assert.New(t)

	defaultConceptsSources := buildDefaultConceptSources()

	tests := []struct {
		testName                     string
		expectedUUIDs                []string
		internalConcordancesConcepts map[string]Concept
		flags                        map[string]string
	}{
		{
			testName: "withoutFlags",
			expectedUUIDs: []string{
				"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
				"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
				"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
			},
			internalConcordancesConcepts: map[string]Concept{
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": Concept{
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": Concept{
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": Concept{
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: defaultConceptsSources,
		},
		{
			testName: "allTmeFlags",
			expectedUUIDs: []string{
				"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
				"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
				"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
			},
			internalConcordancesConcepts: map[string]Concept{
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": Concept{
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": Concept{
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": Concept{
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				FilteringSourcePerson:       TmeSource,
				FilteringSourceLocation:     TmeSource,
				FilteringSourceOrganisation: TmeSource,
			},
		},
		{
			testName: "allCesFlags",
			expectedUUIDs: []string{
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
				"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a385",
				"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
			},
			internalConcordancesConcepts: map[string]Concept{
				"f758ef56-c40a-3162-91aa-3e8a3aabc495": Concept{
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a385": Concept{
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a385",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a385",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a55": Concept{
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				FilteringSourcePerson:       CesSource,
				FilteringSourceLocation:     CesSource,
				FilteringSourceOrganisation: CesSource,
			},
		},
		{
			testName: "cesPeople",
			expectedUUIDs: []string{
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
				"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
				"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a385",
			},
			internalConcordancesConcepts: map[string]Concept{
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": Concept{
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a385": Concept{
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a385",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a385",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": Concept{
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				FilteringSourcePerson:       CesSource,
				FilteringSourceLocation:     TmeSource,
				FilteringSourceOrganisation: TmeSource,
			},
		},
		{
			testName: "cesLocation",
			expectedUUIDs: []string{
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
				"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
				"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
			},
			internalConcordancesConcepts: map[string]Concept{
				"f758ef56-c40a-3162-91aa-3e8a3aabc495": Concept{
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": Concept{
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": Concept{
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				FilteringSourcePerson:       TmeSource,
				FilteringSourceLocation:     CesSource,
				FilteringSourceOrganisation: TmeSource,
			},
		},
		{
			testName: "cesOrganisation",
			expectedUUIDs: []string{
				"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				"http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
				"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
			},
			internalConcordancesConcepts: map[string]Concept{
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": Concept{
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": Concept{
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a55": Concept{
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				FilteringSourcePerson:       TmeSource,
				FilteringSourceLocation:     TmeSource,
				FilteringSourceOrganisation: CesSource,
			},
		},
	}

	for _, testCase := range tests {
		falconHTTPMock := new(mockHttpClient)
		falconHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(sampleFalconResponse)),
			StatusCode: http.StatusOK,
		}, nil)

		ontotextHTTPMock := new(mockHttpClient)
		ontotextHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(sampleOntotextResponse)),
			StatusCode: http.StatusOK,
		}, nil)

		mockClient := new(mockHttpClient)

		internalConcordancesResponse := ConcordanceResponse{
			Concepts: testCase.internalConcordancesConcepts,
		}
		expectedBody, err := json.Marshal(internalConcordancesResponse)
		expect.NoError(err)
		buffer := &ClosingBuffer{
			Buffer: bytes.NewBuffer(expectedBody),
		}
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			Body:       buffer,
			StatusCode: http.StatusOK,
		}, nil)

		mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)

		falconSuggester := NewFalconSuggester("falconHost", "/content/suggest/falcon", falconHTTPMock)
		ontotextSuggester := NewOntotextSuggester("ontotextHost", "/content/suggest/ontotext", ontotextHTTPMock)
		aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, falconSuggester, ontotextSuggester)

		suggestionResp, err := aggregateSuggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: testCase.flags})
		expect.NoError(err)

		actualSuggestions := suggestionResp.Suggestions
		expect.NotNilf(actualSuggestions, "%s -> nil suggestions", testCase.testName)
		expect.Lenf(actualSuggestions, len(testCase.expectedUUIDs), "%s -> unexpected number of suggestions", testCase.testName)

		for _, expectedID := range testCase.expectedUUIDs {
			found := false
			for _, actualSugg := range actualSuggestions {
				if expectedID == actualSugg.ID {
					found = true
					break
				}
			}
			if !found {
				expect.Failf("expected suggestion not returned", "%s -> %s missing", testCase.testName, expectedID)
			}
		}
	}
}

// This is about the other concept types that should be handled as organisations (PublicCompany, PrivateCompany, Company)
func TestAggregateSuggester_GetSuggestionsAllOrganisationsTypes(t *testing.T) {
	expect := assert.New(t)

	falconSuggestions := `{  
		"suggestions":[  
		  {  
			"id":"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
			"apiUrl":"http://api.ft.com/things/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
			"prefLabel":"London Politics",
			"type":"http://www.ft.com/ontology/Topic"
		  },
		  {  
			"id":"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
			"apiUrl":"http://api.ft.com/things/9332270e-f959-3f55-9153-d30acd0d0a50",
			"prefLabel":"Apple",
			"type":"http://www.ft.com/ontology/organisation/Organisation"
		  },
		  {  
			"id":"http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3990",
			"apiUrl":"http://api.ft.com/things/ec0307f0-caf7-11e8-a9b5-6c96cfdf3990",
			"prefLabel":"Facebook",
			"type":"http://www.ft.com/ontology/company/PrivateCompany"
		  },
		  {  
			"id":"http://www.ft.com/thing/0767212c-caf9-11e8-b2ed-6c96cfdf3990",
			"apiUrl":"http://api.ft.com/things/0767212c-caf9-11e8-b2ed-6c96cfdf3990",
			"prefLabel":"CIA",
			"type":"http://www.ft.com/ontology/company/PublicCompany"
		  },
		  {  
			"id":"http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3990",
			"apiUrl":"http://api.ft.com/things/70121ea5-caf8-11e8-b0db-6c96cfdf3990",
			"prefLabel":"ABC",
			"type":"http://www.ft.com/ontology/company/Company"
		  }
		]
	  }`
	ontotextSuggestions := `{  
		"suggestions":[
		  {  
			"id":"http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
			"apiUrl":"http://api.ft.com/things/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
			"prefLabel":"CIA",
			"type":"http://www.ft.com/ontology/company/PublicCompany"
		  },
		  {  
			"id":"http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
			"apiUrl":"http://api.ft.com/things/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
			"prefLabel":"ABC",
			"type":"http://www.ft.com/ontology/company/Company"
		  }
		]
	  }`
	defaultConceptsSources := buildDefaultConceptSources()

	tests := []struct {
		testName                     string
		expectedUUIDs                []string
		flags                        map[string]string
		internalConcordancesConcepts map[string]Concept
	}{
		{
			testName: "withoutFlags",
			flags:    defaultConceptsSources,
			expectedUUIDs: []string{
				"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				"http://www.ft.com/thing/0767212c-caf9-11e8-b2ed-6c96cfdf3990",
				"http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3990",
				"http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3990",
			},
			internalConcordancesConcepts: map[string]Concept{
				"70121ea5-caf8-11e8-b0db-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3990",
					APIURL:    "http://api.ft.com/things/70121ea5-caf8-11e8-b0db-6c96cfdf3990",
					PrefLabel: "ABC",
					Type:      "http://www.ft.com/ontology/company/Company",
				},
				"ec0307f0-caf7-11e8-a9b5-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3990",
					APIURL:    "http://api.ft.com/things/ec0307f0-caf7-11e8-a9b5-6c96cfdf3990",
					PrefLabel: "Facebook",
					Type:      "http://www.ft.com/ontology/company/PrivateCompany",
				},
				"0767212c-caf9-11e8-b2ed-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/0767212c-caf9-11e8-b2ed-6c96cfdf3990",
					APIURL:    "http://api.ft.com/things/0767212c-caf9-11e8-b2ed-6c96cfdf3990",
					PrefLabel: "CIA",
					Type:      "http://www.ft.com/ontology/company/PublicCompany",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": Concept{
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
		},
		{
			testName: "fromCesOrganisations",
			flags: map[string]string{
				FilteringSourceLocation:     TmeSource,
				FilteringSourcePerson:       TmeSource,
				FilteringSourceOrganisation: CesSource,
			},
			expectedUUIDs: []string{
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				"http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
				"http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
			},
			internalConcordancesConcepts: map[string]Concept{
				"70121ea5-caf8-11e8-b0db-6c96cfdf3995": Concept{
					ID:        "http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
					APIURL:    "http://api.ft.com/things/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
					PrefLabel: "ABC",
					Type:      "http://www.ft.com/ontology/company/Company",
				},
				"ec0307f0-caf7-11e8-a9b5-6c96cfdf3995": Concept{
					ID:        "http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
					APIURL:    "http://api.ft.com/things/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
					PrefLabel: "Facebook",
					Type:      "http://www.ft.com/ontology/company/PrivateCompany",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": Concept{
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
		},
	}

	for _, testCase := range tests {
		falconHTTPMock := new(mockHttpClient)
		falconHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(falconSuggestions)),
			StatusCode: http.StatusOK,
		}, nil)

		ontotextHTTPMock := new(mockHttpClient)
		ontotextHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(ontotextSuggestions)),
			StatusCode: http.StatusOK,
		}, nil)

		mockClient := new(mockHttpClient)
		internalConcordancesResponse := ConcordanceResponse{
			Concepts: testCase.internalConcordancesConcepts,
		}
		expectedBody, err := json.Marshal(internalConcordancesResponse)
		expect.NoError(err)
		buffer := &ClosingBuffer{
			Buffer: bytes.NewBuffer(expectedBody),
		}
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			Body:       buffer,
			StatusCode: http.StatusOK,
		}, nil)

		mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)

		falconSuggester := NewFalconSuggester("falconHost", "/content/suggest/falcon", falconHTTPMock)
		ontotextSuggester := NewOntotextSuggester("ontotextHost", "/content/suggest/ontotext", ontotextHTTPMock)
		aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, falconSuggester, ontotextSuggester)

		suggestionResp, err := aggregateSuggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: testCase.flags})
		expect.NoError(err)

		actualSuggestions := suggestionResp.Suggestions
		expect.NotNilf(actualSuggestions, "%s -> nil suggestions", testCase.testName)
		expect.Lenf(actualSuggestions, len(testCase.expectedUUIDs), "%s -> unexpected number of suggestions", testCase.testName)

		for _, expectedID := range testCase.expectedUUIDs {
			found := false
			for _, actualSugg := range actualSuggestions {
				if expectedID == actualSugg.ID {
					found = true
					break
				}
			}
			if !found {
				expect.Failf("expected suggestion not returned", "%s -> %s missing", testCase.testName, expectedID)
			}
		}
	}
}

func TestAggregateSuggester_InternalConcordancesUnavailable(t *testing.T) {
	expect := assert.New(t)
	suggestionAPI := new(mockSuggestionApi)
	falconSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "falcon-suggestion-api",
				APIURL:     "apiurl1",
				PrefLabel:  "prefLabel1",
				Type:       ontologyPersonType,
			},
		},
	}}
	authorsSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       ontologyPersonType,
			},
		},
	}}
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(falconSuggestion, nil).Once()
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()

	defaultConceptsSources := buildDefaultConceptSources()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["falcon-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "falcon-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: ontologyPersonType,
	}

	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: ioutil.NopCloser(strings.NewReader(""))}, fmt.Errorf("error during calling internal concordances"))

	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, suggestionAPI, suggestionAPI)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.Error(err)
	expect.Equal(err.Error(), "error during calling internal concordances")
	expect.Len(response.Suggestions, 0)

	suggestionAPI.AssertExpectations(t)
}

func TestAggregateSuggester_InternalConcordancesUnexpectedStatus(t *testing.T) {
	expect := assert.New(t)
	suggestionAPI := new(mockSuggestionApi)
	falconSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "falcon-suggestion-api",
				APIURL:     "apiurl1",
				PrefLabel:  "prefLabel1",
				Type:       ontologyPersonType,
			},
		},
	}}
	authorsSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       ontologyPersonType,
			},
		},
	}}
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(falconSuggestion, nil)
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil)

	defaultConceptsSources := buildDefaultConceptSources()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["falcon-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "falcon-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: ontologyPersonType,
	}

	mockClient := new(mockHttpClient)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, suggestionAPI, suggestionAPI)

	// 503
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusServiceUnavailable,
	}, nil).Once()
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})
	expect.Error(err)
	expect.Equal("non 200 status code returned: 503", err.Error())
	expect.Len(response.Suggestions, 0)

	// 400
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusBadRequest,
	}, nil).Once()
	response, err = aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})
	expect.Error(err)
	expect.Equal("non 200 status code returned: 400", err.Error())
	expect.Len(response.Suggestions, 0)

	suggestionAPI.AssertExpectations(t)
}

func TestOntotext_MissingDefaultValues(t *testing.T) {
	expect := assert.New(t)

	ontotextHTTPMock := new(mockHttpClient)
	ontotextHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(sampleOntotextResponse)),
		StatusCode: http.StatusOK,
	}, nil)

	suggester := NewOntotextSuggester("ontotextURL", "ontotextEndpoint", ontotextHTTPMock)
	resp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: map[string]string{
		FilteringSourceLocation:     TmeSource,
		FilteringSourceOrganisation: TmeSource,
	}})

	expect.Error(err)
	expect.Equal("No source defined for personSource", err.Error())

	expect.NotNil(resp)
	expect.Len(resp.Suggestions, 0)
}

func TestFalcon_MissingDefaultValues(t *testing.T) {
	expect := assert.New(t)

	falconHTTPMock := new(mockHttpClient)
	falconHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(sampleFalconResponse)),
		StatusCode: http.StatusOK,
	}, nil)

	suggester := NewFalconSuggester("falconURL", "falconEndpoint", falconHTTPMock)
	resp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: map[string]string{
		FilteringSourceLocation:     CesSource,
		FilteringSourceOrganisation: CesSource,
	}})

	expect.Error(err)
	expect.Equal("No source defined for personSource", err.Error())

	expect.NotNil(resp)
	expect.Len(resp.Suggestions, 0)
}

func TestOntotext_ErrorFromService(t *testing.T) {
	expect := assert.New(t)

	ontotextHTTPMock := new(mockHttpClient)
	ontotextHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, fmt.Errorf("Error from ontotext-suggestion-api"))

	suggester := NewOntotextSuggester("ontotextURL", "ontotextEndpoint", ontotextHTTPMock)
	resp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: map[string]string{
		FilteringSourceLocation:     CesSource,
		FilteringSourceOrganisation: TmeSource,
	}})

	expect.Error(err)
	expect.Equal("Error from ontotext-suggestion-api", err.Error())

	expect.NotNil(resp)
	expect.Len(resp.Suggestions, 0)
}

func TestHasFlag(t *testing.T) {
	expect := assert.New(t)

	firstSuggesterTargetedTypes := []string{FilteringSourcePerson, FilteringSourceOrganisation}
	secondSuggesterTargetedTypes := []string{FilteringSourceLocation}
	sourceFlags := SourceFlags{
		Flags: map[string]string{
			FilteringSourceLocation:     CesSource,
			FilteringSourceOrganisation: CesSource,
			FilteringSourcePerson:       CesSource,
		},
	}
	expect.True(sourceFlags.hasFlag(CesSource, firstSuggesterTargetedTypes))
	expect.True(sourceFlags.hasFlag(CesSource, secondSuggesterTargetedTypes))

	sourceFlags.Flags[FilteringSourceLocation] = TmeSource
	expect.True(sourceFlags.hasFlag(CesSource, firstSuggesterTargetedTypes))
	expect.False(sourceFlags.hasFlag(CesSource, secondSuggesterTargetedTypes))
}

func buildDefaultConceptSources() map[string]string {
	defaultConceptsSource := map[string]string{}
	for _, conceptType := range FilteringSources {
		defaultConceptsSource[conceptType] = TmeSource
	}
	return defaultConceptsSource
}
