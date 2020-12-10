package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tidutils "github.com/Financial-Times/transactionid-utils-go"

	"github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/public-suggestions-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	personType = "http://www.ft.com/ontology/person/Person"
)

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

func (s *mockSuggesterService) GetSuggestions(payload []byte, tid string) (service.SuggestionsResponse, error) {
	args := s.Called(payload, tid)
	return args.Get(0).(service.SuggestionsResponse), args.Error(1)
}

func (s *mockSuggesterService) FilterSuggestions(suggestions []service.Suggestion) []service.Suggestion {
	args := s.Called(suggestions)
	return args.Get(0).([]service.Suggestion)
}

func (s *mockSuggesterService) GetName() string {
	return "Mock suggester service"
}

func (s *mockSuggesterService) Check() v1_1.Check {
	args := s.Called()
	return args.Get(0).(v1_1.Check)
}

type allowFilterMock struct {
}

func (m *allowFilterMock) IsConceptAllowed(uuid string) bool {
	return true
}
func TestRequestHandler_HandleSuggestionSuccessfully(t *testing.T) {
	expect := assert.New(t)

	body := []byte(`{"id":"http://www.ft.com/thing/9d5e441e-0b02-11e8-8eb7-42f857ea9f0","byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	expectedResp := service.SuggestionsResponse{Suggestions: []service.Suggestion{
		{
			Concept: service.Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       personType,
			},
		},
	}}
	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockPublicThings := new(mockHttpClient)
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
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: buffer, StatusCode: http.StatusOK}, nil)

	mockPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: ioutil.NopCloser(strings.NewReader("")), StatusCode: http.StatusOK}, nil)
	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	mockSuggester.On("GetSuggestions", body, "tid_test").Return(expectedResp, nil).Once()
	mockSuggester.On("FilterSuggestions", expectedResp.Suggestions, mock.Anything).Return(expectedResp.Suggestions).Once()
	mockSuggester.On("GetSuggestions", body, "tid_test").Return(service.SuggestionsResponse{}, nil)

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[{"id":"authors-suggestion-api","apiUrl":"apiurl2","type":"http://www.ft.com/ontology/person/Person","prefLabel":"prefLabel2","isFTAuthor":true}]}`, w.Body.String())

	mockSuggester.AssertExpectations(t)
	mockPublicThings.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestRequestHandler_HandleSuggestionErrorOnRequestRead(t *testing.T) {
	expect := assert.New(t)

	req := httptest.NewRequest("POST", "/content/suggest", &faultyReader{})
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockPublicThings := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Error while reading payload"}`, w.Body.String())

	mockSuggester.AssertExpectations(t)    //no calls
	mockPublicThings.AssertExpectations(t) // no calls
	mockClient.AssertExpectations(t)       //no calls
}

func TestRequestHandler_HandleSuggestionEmptyBody(t *testing.T) {
	expect := assert.New(t)

	body := []byte("")
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockPublicThings := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Payload should be a non-empty JSON object"}`, w.Body.String())

	mockSuggester.AssertExpectations(t)    //no calls
	mockPublicThings.AssertExpectations(t) //no calls
	mockClient.AssertExpectations(t)       //no calls
}

func TestRequestHandler_HandleSuggestionEmptyJsonRequest(t *testing.T) {
	expect := assert.New(t)

	body := []byte("{}")
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockPublicThings := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusBadRequest, w.Code)
	expect.Equal(`{"message": "Payload should be a non-empty JSON object"}`, w.Body.String())

	mockSuggester.AssertExpectations(t)    //no calls
	mockPublicThings.AssertExpectations(t) //no calls
	mockClient.AssertExpectations(t)       //no calls
}

func TestRequestHandler_HandleSuggestionErrorOnGetSuggestions(t *testing.T) {
	expect := assert.New(t)

	body := []byte(`{"bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockPublicThings := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	mockSuggester.On("GetSuggestions", body, "tid_test").Return(service.SuggestionsResponse{Suggestions: []service.Suggestion{}}, errors.New("timeout error"))

	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())

	mockSuggester.AssertExpectations(t)
	mockPublicThings.AssertExpectations(t) //no calls
	mockClient.AssertExpectations(t)       //no calls
}

func TestRequestHandler_HandleSuggestionOkWhenNoContentSuggestions(t *testing.T) {
	expect := assert.New(t)
	body := []byte(`{"bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockPublicThings := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	service.NoContentError = errors.New("No content error")
	mockSuggester.On("GetSuggestions", body, "tid_test").Return(service.SuggestionsResponse{
		Suggestions: make([]service.Suggestion, 0),
	}, service.NoContentError)

	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())

	mockSuggester.AssertExpectations(t)
	mockPublicThings.AssertExpectations(t) //no calls
	mockClient.AssertExpectations(t)       //no calls
}

//Might not happen at all if MetadataServices returns always 204 when there are no suggestions
func TestRequestHandler_HandleSuggestionOkWhenEmptySuggestions(t *testing.T) {
	expect := assert.New(t)
	body := []byte(`{"byline":"Test byline","bodyXML":"Test body","title":"Test title"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockPublicThings := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	mockSuggester.On("GetSuggestions", body, "tid_test").Return(service.SuggestionsResponse{Suggestions: []service.Suggestion{}}, nil)

	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusOK, w.Code)
	expect.Equal(`{"suggestions":[]}`, w.Body.String())

	mockSuggester.AssertExpectations(t)
	mockPublicThings.AssertExpectations(t) //no calls
	mockClient.AssertExpectations(t)       //no calls
}

func TestRequestHandler_HandleSuggestionErrorOnGetConcordance(t *testing.T) {
	expect := assert.New(t)

	body := []byte(`{"bodyXML":"Test body"}`)
	req := httptest.NewRequest("POST", "/content/suggest", bytes.NewReader(body))
	req.Header.Add("X-Request-Id", "tid_test")
	w := httptest.NewRecorder()

	log := logger.NewUPPLogger("test-logger", "panic")
	mockClient := new(mockHttpClient)
	mockSuggester := new(mockSuggesterService)
	mockPublicThings := new(mockHttpClient)
	mockConcordance := &service.ConcordanceService{ConcordanceBaseURL: "concordanceBaseURL", ConcordanceEndpoint: "concordanceEndpoint", Client: mockClient}

	mockSuggester.On("GetSuggestions", body, "tid_test").Return(service.SuggestionsResponse{Suggestions: []service.Suggestion{
		{
			Concept: service.Concept{
				IsFTAuthor: true,
				ID:         "authors-suggestion-api",
				APIURL:     "apiurl2",
				PrefLabel:  "prefLabel2",
				Type:       personType,
			},
		},
	}}, nil)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("timeout error"))

	broaderService := &service.BroaderConceptsProvider{
		Client: mockPublicThings,
	}

	handler := NewRequestHandler(service.NewAggregateSuggester(log, mockConcordance, broaderService, &allowFilterMock{}, mockSuggester), log)
	handler.HandleSuggestion(w, req)

	expect.Equal(http.StatusServiceUnavailable, w.Code)
	expect.Equal(`{"message": "aggregating suggestions failed!"}`, w.Body.String())

	mockSuggester.AssertExpectations(t)
	mockPublicThings.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestHandleRefreshFilterCache(t *testing.T) {
	tests := map[string]struct {
		Response     string
		ResponseCode int
		ExpectError  bool
		AllowedUUIDs map[string]bool
	}{
		"success": {
			Response:     `{"uuids":["blocked"]}`,
			ResponseCode: http.StatusOK,
			AllowedUUIDs: map[string]bool{"blocked": false, "allowed": true},
		},
		"blacklister internal fail": {
			ResponseCode: http.StatusInternalServerError,
			ExpectError:  true,
		},
		"broken body": {
			Response:     `{"broken":`,
			ResponseCode: http.StatusOK,
			ExpectError:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := &http.Client{
				Transport: &transportMock{handle: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, "http://example.com/blacklist?refresh=true", req.URL.String())

					tid := req.Header.Get(tidutils.TransactionIDHeader)
					assert.Equal(t, "tid_test", tid)
					agent := req.Header.Get("User-Agent")
					assert.Equal(t, "UPP public-suggestions-api", agent)

					res := httptest.NewRecorder()
					res.Code = test.ResponseCode
					res.Body.WriteString(test.Response)
					return res.Result(), nil
				}},
			}

			filter := service.NewCachedConceptFilter("http://example.com", "/blacklist", client)

			err := filter.RefreshCache(context.TODO(), "tid_test")
			if test.ExpectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			for id, allowed := range test.AllowedUUIDs {
				assert.Equal(t, allowed, filter.IsConceptAllowed(id))
			}
		})
	}
}

type transportMock struct {
	handle func(r *http.Request) (*http.Response, error)
}

func (m *transportMock) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.handle == nil {
		return nil, errors.New("handler not implemented")
	}
	return m.handle(req)
}
