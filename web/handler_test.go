package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"errors"

	"github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/public-suggestions-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	log.InitLogger("handler_test", "ERROR")
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
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"

	body := []byte(`{"byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	expectedResp := service.SuggestionsResponse{Suggestions: []service.Suggestion{{PrefLabel: "TestMan"}}}
	mockSuggester := new(mockSuggesterService)
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(expectedResp, nil).Once()
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{}, nil)

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[{"prefLabel":"TestMan"}]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionSuccessfullyWithAuthorsTME(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"
	body := []byte(`{"byline":"Test byline","bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest?source=tme", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource}}).Return(service.SuggestionsResponse{[]service.Suggestion{{PrefLabel: "TmePrefLabel"}}}, nil).Once()

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[{"prefLabel":"TmePrefLabel"}]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionErrorOnRequestRead(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"

	req := httptest.NewRequest("POST", "/content/suggest", &faultyReader{})
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Error while reading payload"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionEmptyBody(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"
	body := []byte("")
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Payload should be a non-empty JSON object"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionEmptyJsonRequest(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"
	body := []byte("{}")
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Payload should be a non-empty JSON object"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionErrorOnGetSuggestions(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"
	body := []byte(`{"bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{Suggestions: []service.Suggestion{}}, errors.New("Timeout error"))

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionErrorInvalidSourceParamOnGetSuggestions(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"
	body := []byte(`{"byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest?source=invalid", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "source flag incorrectly set"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}

func TestRequestHandler_HandleSuggestionOkWhenNoContentSuggestions(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"
	body := []byte(`{"bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)
	service.NoContentError = errors.New("No content error")
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{make([]service.Suggestion, 0)}, service.NoContentError)

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

//Might not happen at all if MetadataServices returns always 204 when there are no suggestions
func TestRequestHandler_HandleSuggestionOkWhenEmptySuggestions(t *testing.T) {
	expect := assert.New(t)
	ConcordanceApiBaseURL := "http://internal-concordances-api:8080"
	ConcordanceEndpoint := "/internalconcordances"
	body := []byte(`{"byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)
	mockSuggester.On("GetSuggestions", body, "tid_test", service.SourceFlags{Flags: []string{service.TmeSource, service.AuthorsSource}}).Return(service.SuggestionsResponse{Suggestions: []service.Suggestion{}}, nil)

	handler := NewRequestHandler(&service.AggregateSuggester{ConcordanceApiBaseURL, ConcordanceEndpoint, []service.Suggester{mockSuggester}})
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}
