package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"errors"

	"encoding/json"

	"github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/public-suggestions-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	log.InitLogger("handler_test", "ERROR")
}

type mockHttpClient struct {
	mock.Mock
}

func (c *mockHttpClient) Do(req *http.Request) (resp *http.Response, err error) {
	args := c.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

type ClosingBuffer struct {
	*bytes.Buffer
}

func (cb *ClosingBuffer) Close() (err error) {
	// do nothing
	return
}

type mockSuggesterService struct {
	mock.Mock
}

type faultyReader struct {
}

func (r *faultyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("Reading error")
}

func (r *faultyReader) Close() error {
	return nil
}

func (s *mockSuggesterService) GetSuggestions(payload []byte, tid string, flags service.SourceFlags) (service.SuggestionsResponse, error) {
	args := s.Called(payload, tid, flags)
	return args.Get(0).(service.SuggestionsResponse), args.Error(1)
}

func (s *mockSuggesterService) GetName() string {
	return "Mock suggester service"
}

func (s *mockSuggesterService) Check() v1_1.Check {
	args := s.Called()
	return args.Get(0).(v1_1.Check)
}

func TestRequestHandler_HandleSuggestionSuccessfully(t *testing.T) {
	expect := assert.New(t)

	body := []byte(`{"byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()
	personType := "http://www.ft.com/ontology/person/Person"
	expectedResp := service.SuggestionsResponse{Suggestions: []service.Suggestion{
		{IsFTAuthor: true, Id: "authors-suggestion-api", ApiUrl: "apiurl2", PrefLabel: "prefLabel2", SuggestionType: personType},
	}}

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	mockInternalConcResp := service.ConcordanceResponse{
		Concepts: make(map[string]service.Concept),
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = service.Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: personType,
	}
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer}, nil)

	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(expectedResp, nil).Once()
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{}, nil)
	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[{"id":"authors-suggestion-api","apiUrl":"apiurl2","prefLabel":"prefLabel2","type":"http://www.ft.com/ontology/person/Person","isFTAuthor":true}]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionSuccessfullyWithAuthorsTME(t *testing.T) {
	expect := assert.New(t)

	body := []byte(`{"byline":"Test byline","bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest?source=tme", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	personType := "http://www.ft.com/ontology/person/Person"
	expectedResp := service.SuggestionsResponse{Suggestions: []service.Suggestion{
		{IsFTAuthor: false, Id: "falcon-suggestion-api", ApiUrl: "apiurl1", PrefLabel: "prefLabel1", SuggestionType: personType},
	}}

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	mockInternalConcResp := service.ConcordanceResponse{
		Concepts: make(map[string]service.Concept),
	}
	mockInternalConcResp.Concepts["falcon-suggestion-api"] = service.Concept{
		IsFTAuthor: false, ID: "falcon-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: personType,
	}
	expectedBody, err := json.Marshal(&mockInternalConcResp)
	require.NoError(t, err)
	buffer := &ClosingBuffer{
		Buffer: bytes.NewBuffer(expectedBody),
	}

	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer}, nil)
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource}}).Return(expectedResp, nil).Once()

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[{"id":"falcon-suggestion-api","apiUrl":"apiurl1","prefLabel":"prefLabel1","type":"http://www.ft.com/ontology/person/Person"}]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionErrorOnRequestRead(t *testing.T) {
	expect := assert.New(t)

	req := httptest.NewRequest("POST", "/content/suggest", &faultyReader{})
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Error while reading payload"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionEmptyBody(t *testing.T) {
	expect := assert.New(t)

	body := []byte("")
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Payload should be a non-empty JSON object"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionEmptyJsonRequest(t *testing.T) {
	expect := assert.New(t)

	body := []byte("{}")
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Payload should be a non-empty JSON object"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionErrorOnGetSuggestions(t *testing.T) {
	expect := assert.New(t)

	body := []byte(`{"bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockClient := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}
	mockSuggester := new(mockSuggesterService)

	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{Suggestions: []service.Suggestion{}}, errors.New("Timeout error"))

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionErrorInvalidSourceParamOnGetSuggestions(t *testing.T) {
	expect := assert.New(t)
	body := []byte(`{"byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest?source=invalid", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "source flag incorrectly set"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionOkWhenNoContentSuggestions(t *testing.T) {
	expect := assert.New(t)
	body := []byte(`{"bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	service.NoContentError = errors.New("No content error")
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{make([]service.Suggestion, 0)}, service.NoContentError)

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

//Might not happen at all if MetadataServices returns always 204 when there are no suggestions
func TestRequestHandler_HandleSuggestionOkWhenEmptySuggestions(t *testing.T) {
	expect := assert.New(t)
	body := []byte(`{"byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{Suggestions: []service.Suggestion{}}, nil)

	handler := NewRequestHandler(&service.AggregateSuggester{mockConcordance, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}
