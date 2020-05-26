package service

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestConcordanceService_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200).Once()
	server := mockServer.startMockServer(t)
	defer server.Close()

	suggester := NewConcordance(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("internal-concordances", check.ID)
	expect.Equal("Suggestions won't work", check.BusinessImpact)
	expect.Equal("internal-concordances Healthcheck", check.Name)
	expect.Equal("https://runbooks.in.ft.com/internal-concordances", check.PanicGuide)
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
	defer server.Close()

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
	var urlErr *url.Error
	if expect.True(errors.As(err, &urlErr)) {
		expect.Equal("parse", urlErr.Op)
	}
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
