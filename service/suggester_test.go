package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestFalconSuggester_GetSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	expectedSuggestions := []Suggestion{
		Suggestion{
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

	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test", SourceFlags{Flags: []string{TmeSource}})

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.True(len(actualSuggestions) == len(expectedSuggestions))

	for _, expected := range expectedSuggestions {
		expect.Contains(actualSuggestions, expected)
	}
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_GetSuggestionsWithServiceUnavailable(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", []byte("{}"), "tid_test", "application/json", "application/json").Return(http.StatusServiceUnavailable, nil)
	server := mockServer.startMockServer(t)

	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: []string{TmeSource}})

	expect.Error(err)
	expect.Equal("Falcon Suggestion API returned HTTP 503", err.Error())
	expect.Nil(suggestionResp.Suggestions)

	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_GetSuggestionsErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)
	suggester := NewFalconSuggester(":/", "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: []string{TmeSource}})

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("parse ://content/suggest: missing protocol scheme", err.Error())
}

func TestFalconSuggester_GetSuggestionsErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewFalconSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: []string{TmeSource}})

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

	suggester := NewFalconSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: []string{TmeSource}})

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

	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test", SourceFlags{Flags: []string{TmeSource}})

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

func TestAggregateSuggester_GetSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	suggestionApi := new(mockSuggestionApi)
	mockClient := new(mockHttpClient)
	mockConcordance := &ConcordanceService{"internal-concordances", "internal-concordances", "ConcordanceBaseURL", "ConcordanceEndpoint", mockClient, "Suggestions won't work"}

	falconSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		Suggestion{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "falcon-suggestion-api",
				APIURL:     "apiurl1",
				PrefLabel:  "prefLabel1",
				Type:       personType},
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
				Type:       personType},
		},
	},
	}
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(falconSuggestion, nil).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["falcon-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "falcon-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: personType,
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: personType,
	}
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer}, nil)

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	broaderConceptsProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)
	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderConceptsProvider, suggestionApi, suggestionApi)
	response, _ := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: []string{AuthorsSource}})

	expect.Len(response.Suggestions, 2)

	expect.Contains(response.Suggestions, falconSuggestion.Suggestions[0])
	expect.Contains(response.Suggestions, authorsSuggestion.Suggestions[0])

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetEmptySuggestionsArrayIfNoAggregatedSuggestionAvailable(t *testing.T) {
	expect := assert.New(t)
	suggestionApi := new(mockSuggestionApi)
	mockConcordance := new(ConcordanceService)
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{}, errors.New("Falcon err"))

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)
	broaderConceptsProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)
	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderConceptsProvider, suggestionApi, suggestionApi)
	response, _ := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: []string{AuthorsSource}})

	expect.Len(response.Suggestions, 0)
	expect.NotNil(response.Suggestions)

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetSuggestionsNoErrorForFalconSuggestionApi(t *testing.T) {
	expect := assert.New(t)
	suggestionApi := new(mockSuggestionApi)
	mockClient := new(mockHttpClient)
	mockConcordance := &ConcordanceService{"internal-concordances", "internal-concordances", "ConcordanceBaseURL", "ConcordanceEndpoint", mockClient, "Suggestions won't work"}
	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: personType,
	}
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer}, nil)

	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{}, errors.New("Falcon err")).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{Suggestions: []Suggestion{
		Suggestion{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       personType,
			},
		},
	},
	}, nil).Once()

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)
	broaderConceptsProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)
	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderConceptsProvider, suggestionApi, suggestionApi)
	response, _ := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: []string{AuthorsSource}})

	expect.Len(response.Suggestions, 1)

	expect.Equal(response.Suggestions[0].Concept.ID, "authors-suggestion-api")

	suggestionApi.AssertExpectations(t)
}

func TestGetSuggestions_NoResultsForAuthorsTmeSourceFlag(t *testing.T) {
	expect := assert.New(t)
	suggester := AuthorsSuggester{}
	response, _ := suggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: []string{TmeSource}})

	expect.Equal(response.Suggestions, []Suggestion{})
}

func TestAuthorsSuggester_GetSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	expectedSuggestions := []Suggestion{
		Suggestion{
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

	suggester := NewAuthorsSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test", SourceFlags{Flags: []string{AuthorsSource}})

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
		Suggestion{
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

	suggester := NewFalconSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test", SourceFlags{Flags: []string{TmeSource, AuthorsSource}})

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.Equal(1, len(actualSuggestions))

	for _, expected := range expectedSuggestions {
		expect.Contains(actualSuggestions, expected)
	}
	mock.AssertExpectationsForObjects(t, mockServer)
}
