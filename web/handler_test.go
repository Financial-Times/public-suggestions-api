package web

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"net/http/httptest"
	"net/http"
	"github.com/stretchr/testify/mock"
	"github.com/Financial-Times/draft-suggestion-api/service"
	"github.com/Financial-Times/go-fthealth/v1_1"
	"bytes"
	log "github.com/Financial-Times/go-logger"
)

func init() {
	log.InitLogger("handler_test", "ERROR")
}

type mockSuggesterService struct {
	mock.Mock
}

func (s *mockSuggesterService) GetSuggestions(payload []byte, tid string) (service.SuggestionsResponse, error) {
	args := s.Called(payload, tid)
	return args.Get(0).(service.SuggestionsResponse), args.Error(1)
}

func (s *mockSuggesterService) Check() v1_1.Check {
	args := s.Called()
	return args.Get(0).(v1_1.Check)
}

func TestRequestHandler_HandleSuggestionSuccessfully(t *testing.T) {
	expect := assert.New(t)

	body := []byte(`{"bodyXml": "Test"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	expectedResp := service.SuggestionsResponse{Suggestions: []service.Suggestion{{PrefLabel: "TestMan", }}}
	mockSuggester := new(mockSuggesterService)
	mockSuggester.On("GetSuggestions", body, "tid_test").Return(expectedResp, nil)

	handler := NewRequestHandler(mockSuggester)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[{"prefLabel":"TestMan"}]}`, w.Body.String())
	mockSuggester.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionEmptyBody(t *testing.T) {
	expect := assert.New(t)

	body := []byte("")
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	mockSuggester := new(mockSuggesterService)

	handler := NewRequestHandler(mockSuggester)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Payload should not be empty"}`, w.Body.String())
	mockSuggester.AssertExpectations(t) //no calls
}
