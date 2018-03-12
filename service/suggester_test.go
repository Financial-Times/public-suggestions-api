package service

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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
            "predicate": "http://www.ft.com/ontology/annotation/mentions",
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

type mockFalconSuggestionApiServer struct {
	mock.Mock
}

func (m *mockFalconSuggestionApiServer) startMockServer(t *testing.T) *httptest.Server {
	router := mux.NewRouter()
	router.HandleFunc("/content/suggest", func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		assert.Equal(t, "UPP draft-suggestion-api", ua)

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

func (m *mockFalconSuggestionApiServer) GTG() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockFalconSuggestionApiServer) UploadRequest(body []byte, tid, contentTypeHeader, acceptHeader string) (int, interface{}) {
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
		{
			Predicate:      "http://www.ft.com/ontology/annotation/mentions",
			Id:             "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
			ApiUrl:         "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
			PrefLabel:      "Donald Kaberuka",
			SuggestionType: "http://www.ft.com/ontology/person/Person",
			IsFTAuthor:     false,
		},
		{
			Predicate:      "http://www.ft.com/ontology/annotation/mentions",
			Id:             "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
			ApiUrl:         "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
			PrefLabel:      "Lawrence Summers",
			SuggestionType: "http://www.ft.com/ontology/person/Person",
			IsFTAuthor:     true,
		},
	}

	body, err := json.Marshal(&expectedSuggestions)
	expect.NoError(err)
	mockServer := new(mockFalconSuggestionApiServer)
	mockServer.On("UploadRequest", body, "tid_test", "application/json", "application/json").Return(http.StatusOK, []byte(sampleJSONResponse))
	server := mockServer.startMockServer(t)

	suggester := NewSuggester(server.URL, "/content/suggest", http.DefaultClient)
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

func TestFalconSuggester_GetSuggestionsWithServiceUnavailable(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockFalconSuggestionApiServer)
	mockServer.On("UploadRequest", []byte("{}"), "tid_test", "application/json", "application/json").Return(http.StatusServiceUnavailable, nil)
	server := mockServer.startMockServer(t)

	suggester := NewSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Error(err)
	expect.Equal("Falcon Suggestion API returned HTTP 503, body: ", err.Error())
	expect.Nil(suggestionResp.Suggestions)

	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_GetSuggestionsErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)
	suggester := NewSuggester(":/", "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("parse ://content/suggest: missing protocol scheme", err.Error())
}

func TestFalconSuggester_GetSuggestionsErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

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

	suggester := NewSuggester("http://test-url", "/content/suggest", mockClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Nil(suggestionResp.Suggestions)
	expect.Error(err)
	expect.Equal("Read error", err.Error())
	mockClient.AssertExpectations(t)
	mockBody.AssertExpectations(t)
}

func TestFalconSuggester_GetSuggestionsErrorOnEmptyBodyResponse(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockFalconSuggestionApiServer)
	mockServer.On("UploadRequest", []byte("{}"), "tid_test", "application/json", "application/json").Return(http.StatusOK, []byte{})
	server := mockServer.startMockServer(t)

	suggester := NewSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions([]byte("{}"), "tid_test")

	expect.Error(err)
	expect.Equal("unexpected end of JSON input", err.Error())
	expect.Nil(suggestionResp.Suggestions)

	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockFalconSuggestionApiServer)
	mockServer.On("GTG").Return(200)
	server := mockServer.startMockServer(t)

	suggester := NewSuggester(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("falcon-suggestion-api", check.ID)
	expect.Equal("Suggestions from TME won't work", check.BusinessImpact)
	expect.Equal("Falcon Suggestion API Healthcheck", check.Name)
	expect.Equal("https://dewey.in.ft.com/view/system/draft-suggestion-api", check.PanicGuide)
	expect.Equal("Falcon Suggestion API is not available", check.TechnicalSummary)
	expect.Equal(uint8(2), check.Severity)
	expect.NoError(err)
	expect.Equal("Falcon Suggestion API is healthy", checkResult)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_CheckHealthUnhealthy(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockFalconSuggestionApiServer)
	mockServer.On("GTG").Return(503)
	server := mockServer.startMockServer(t)

	suggester := NewSuggester(server.URL, "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	expect.Empty(checkResult)
	assert.Equal(t, "Health check returned a non-200 HTTP status: 503", err.Error())
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestFalconSuggester_CheckHealthErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	suggester := NewSuggester(":/", "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "parse ://__gtg: missing protocol scheme", err.Error())
	expect.Empty(checkResult)
}

func TestFalconSuggester_CheckHealthErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("Http Client err"))

	suggester := NewSuggester("http://test-url", "/__gtg", mockClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "Http Client err", err.Error())
	expect.Empty(checkResult)
	mockClient.AssertExpectations(t)
}
