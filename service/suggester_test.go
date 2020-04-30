package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func (m *mockSuggestionApi) FilterSuggestions(suggestions []Suggestion) []Suggestion {
	args := m.Called(suggestions)
	return args.Get(0).([]Suggestion)
}

func (m *mockSuggestionApi) GetSuggestions(payload []byte, tid string) (SuggestionsResponse, error) {
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
		defer r.Body.Close()
		assert.NoError(t, err)
		assert.True(t, len(body) > 0)

		respStatus, respBody := m.UploadRequest(body, tid, contentTypeHeader, acceptHeader)
		w.WriteHeader(respStatus)
		if respBody != nil {
			_, _ = w.Write(respBody.([]byte))
		}
	}).Methods(http.MethodPost)

	router.HandleFunc("/__gtg", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(m.GTG())
		r.Body.Close()
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

func TestOntotextSuggester_GetSuggestionsSuccessfullyWithoutAuthors(t *testing.T) {
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
	defer server.Close()

	suggester := NewOntotextSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test")
	suggestionResp.Suggestions = suggester.FilterSuggestions(suggestionResp.Suggestions)

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.Len(actualSuggestions, 1)

	expect.Contains(actualSuggestions, expectedSuggestions[0])
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestAuthorsSuggester_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200)
	server := mockServer.startMockServer(t)
	defer server.Close()

	suggester := NewAuthorsSuggester(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("authors-suggestion-api", check.ID)
	expect.Equal("Suggesting authors from Concept Search won't work", check.BusinessImpact)
	expect.Equal("Authors Suggestion API Healthcheck", check.Name)
	expect.Equal("https://runbooks.in.ft.com/authors-suggestion-api", check.PanicGuide)
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
	defer server.Close()

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
	var urlErr *url.Error
	if expect.True(errors.As(err, &urlErr)) {
		expect.Equal("parse", urlErr.Op)
	}
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

func TestOntotextSuggester_GetSuggestionsWithServiceUnavailable(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", []byte("{}"), "tid_test", "application/json", "application/json").Return(http.StatusServiceUnavailable, nil)
	server := mockServer.startMockServer(t)
	defer server.Close()

	suggester := NewOntotextSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Error(err)
	expect.Equal("Ontotext Suggestion API returned HTTP 503", err.Error())
	expect.Nil(suggestionResp.Suggestions)

	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestOntotextSuggester_GetSuggestionsErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)
	suggester := NewOntotextSuggester(":/", "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Nil(suggestionResp.Suggestions)
	var urlErr *url.Error
	if expect.True(errors.As(err, &urlErr)) {
		expect.Equal("parse", urlErr.Op)
	}
}

func TestOntotextSuggester_GetSuggestionsErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewOntotextSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("Http Client err", err.Error())
	mockClient.AssertExpectations(t)
}

func TestOntotextSuggester_GetSuggestionsErrorOnResponseBodyRead(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockBody := new(mockResponseBody)

	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: mockBody}, nil)
	mockBody.On("Read", mock.AnythingOfType("[]uint8")).Return(0, errors.New("Read error"))
	mockBody.On("Close").Return(nil)

	suggester := NewOntotextSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("Read error", err.Error())
	mockClient.AssertExpectations(t)
	mockBody.AssertExpectations(t)
}

func TestOntotextSuggester_GetSuggestionsErrorOnEmptyBodyResponse(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("UploadRequest", []byte("{}"), "tid_test", "application/json", "application/json").Return(http.StatusOK, []byte{})
	server := mockServer.startMockServer(t)
	defer server.Close()

	suggester := NewOntotextSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Error(err)
	expect.Equal("unexpected end of JSON input", err.Error())
	expect.Nil(suggestionResp.Suggestions)

	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestOntotextSuggester_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200)
	server := mockServer.startMockServer(t)
	defer server.Close()

	suggester := NewOntotextSuggester(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("ontotext-suggestion-api", check.ID)
	expect.Equal("Suggesting locations, organisations and people from Ontotext won't work", check.BusinessImpact)
	expect.Equal("Ontotext Suggestion API Healthcheck", check.Name)
	expect.Equal("https://runbooks.in.ft.com/ontotext-suggestion-api", check.PanicGuide)
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
	defer server.Close()

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

	var urlErr *url.Error
	if expect.True(errors.As(err, &urlErr)) {
		expect.Equal("parse", urlErr.Op)
	}
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
	defer server.Close()

	suggester := NewAuthorsSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test")

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.True(len(actualSuggestions) == len(expectedSuggestions))

	for _, expected := range expectedSuggestions {
		expect.Contains(actualSuggestions, expected)
	}
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestOntotext_ErrorFromService(t *testing.T) {
	expect := assert.New(t)

	ontotextHTTPMock := new(mockHttpClient)
	ontotextHTTPMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, fmt.Errorf("Error from ontotext-suggestion-api"))

	suggester := NewOntotextSuggester("ontotextURL", "ontotextEndpoint", ontotextHTTPMock)
	resp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Error(err)
	expect.Equal("Error from ontotext-suggestion-api", err.Error())

	expect.NotNil(resp)
	expect.Len(resp.Suggestions, 0)
}
