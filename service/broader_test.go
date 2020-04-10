package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBroaderConceptsProvider_CheckHealth(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(200).Once()
	server := mockServer.startMockServer(t)
	defer server.Close()

	suggester := NewBroaderConceptsProvider(server.URL, "/__gtg", http.DefaultClient)
	check := suggester.Check()
	checkResult, err := check.Checker()

	expect.Equal("public-things-api", check.ID)
	expect.Equal("Excluding broader concepts will not work", check.BusinessImpact)
	expect.Equal("public-things-api Healthcheck", check.Name)
	expect.Equal("https://biz-ops.in.ft.com/System/public-things-api", check.PanicGuide)
	expect.Equal("public-things-api is not available", check.TechnicalSummary)
	expect.Equal(uint8(2), check.Severity)
	expect.NoError(err)
	expect.Equal("public-things-api is healthy", checkResult)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestBroaderConceptsProvider_CheckHealthUnhealthy(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockSuggestionApiServer)
	mockServer.On("GTG").Return(503)
	server := mockServer.startMockServer(t)
	defer server.Close()

	suggester := NewBroaderConceptsProvider(server.URL, "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	expect.Empty(checkResult)
	assert.Equal(t, "Health check returned a non-200 HTTP status: 503", err.Error())
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestBroaderConceptsProvider_CheckHealthErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	suggester := NewBroaderConceptsProvider(":/", "/__gtg", http.DefaultClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "parse \"://__gtg\": missing protocol scheme", err.Error())
	expect.Empty(checkResult)
}

func TestBroaderConceptsProvider_CheckHealthErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("http client err"))

	suggester := NewBroaderConceptsProvider("http://test-url", "/__gtg", mockClient)
	checkResult, err := suggester.Check().Checker()

	expect.Error(err)
	assert.Equal(t, "http client err", err.Error())
	expect.Empty(checkResult)
	mockClient.AssertExpectations(t)
}

func TestBroaderService_excludeBroaderConcepts(t *testing.T) {
	ast := assert.New(t)

	testCases := []struct {
		testName    string
		suggestions map[int][]Suggestion

		expectedSuggestions   map[int][]Suggestion
		expectedErrorContains string

		publicThingsResponse    broaderResponse
		publicThingsStatusCode  int
		publicThingsError       error
		publicThingsCallSkipped bool
	}{
		{
			testName: "ok_NoExclude",
			publicThingsResponse: broaderResponse{
				Things: map[string]Thing{
					"6f14ea94-690f-3ed4-98c7-b926683c735a": {
						BroaderConcepts: []BroaderConcept{
							{
								ID: "http://www.ft.com/thing/993cce16-dcf8-11e8-950b-6c96cfdf3997",
							},
						},
					},
				},
			},
			suggestions: map[int][]Suggestion{
				1: {
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
				},
			},
			expectedSuggestions: map[int][]Suggestion{
				1: {
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
				},
			},
		},
		{
			testName: "ok_Exclude",
			publicThingsResponse: broaderResponse{
				Things: map[string]Thing{
					"6f14ea94-690f-3ed4-98c7-b926683c735a": {
						BroaderConcepts: []BroaderConcept{
							{
								ID: "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							},
						},
					},
				},
			},
			suggestions: map[int][]Suggestion{
				1: {
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
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							PrefLabel:  "Donald Kaberuka",
							Type:       "http://www.ft.com/ontology/person/Person",
							IsFTAuthor: false,
						},
					},
				},
			},
			expectedSuggestions: map[int][]Suggestion{
				1: {
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
				},
			},
		},
		{
			testName: "ok_noSuggestionsReceived",
			publicThingsResponse: broaderResponse{
				Things: map[string]Thing{},
			},
			suggestions:             map[int][]Suggestion{},
			expectedSuggestions:     map[int][]Suggestion{},
			publicThingsCallSkipped: true,
		},
		{
			testName: "error_non200",
			publicThingsResponse: broaderResponse{
				Things: map[string]Thing{},
			},
			publicThingsStatusCode: http.StatusBadGateway,
			expectedErrorContains:  "non 200 status code returned: 502",
			suggestions: map[int][]Suggestion{
				1: {
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
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							PrefLabel:  "Donald Kaberuka",
							Type:       "http://www.ft.com/ontology/person/Person",
							IsFTAuthor: false,
						},
					},
				},
			},
			expectedSuggestions: map[int][]Suggestion{
				1: {
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
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							PrefLabel:  "Donald Kaberuka",
							Type:       "http://www.ft.com/ontology/person/Person",
							IsFTAuthor: false,
						},
					},
				},
			},
		},
		{
			testName: "ok_ExcludedMultipleConcepts",
			publicThingsResponse: broaderResponse{
				Things: map[string]Thing{
					"a13db394-dd01-11e8-a00d-6c96cfdf3997": {
						BroaderConcepts: []BroaderConcept{
							{
								ID: "http://www.ft.com/thing/a1a2f869-dd01-11e8-abd7-6c96cfdf3997",
							},
						},
					},
					"a1a2f869-dd01-11e8-abd7-6c96cfdf3997": {
						BroaderConcepts: []BroaderConcept{
							{
								ID: "http://www.ft.com/thing/a1f96d3d-dd01-11e8-b32e-6c96cfdf3997",
							},
						},
					},
					"ca26a873-dd01-11e8-9a17-6c96cfdf3997": {
						BroaderConcepts: []BroaderConcept{
							{
								ID: "http://www.ft.com/thing/c9e114c1-dd01-11e8-8d2b-6c96cfdf3997",
							},
							{
								ID: "http://www.ft.com/thing/c517ae15-dd01-11e8-aaba-6c96cfdf3997",
							},
						},
					},
					"c9e114c1-dd01-11e8-8d2b-6c96cfdf3997": {
						BroaderConcepts: []BroaderConcept{
							{
								ID: "http://www.ft.com/thing/c517ae15-dd01-11e8-aaba-6c96cfdf3997",
							},
						},
					},
				},
			},
			suggestions: map[int][]Suggestion{
				1: {
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/a13db394-dd01-11e8-a00d-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/a13db394-dd01-11e8-a00d-6c96cfdf3997",
							PrefLabel:  "Bank of England",
							Type:       "http://www.ft.com/ontology/organisation/Organisation",
							IsFTAuthor: false,
						},
					},
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/a1a2f869-dd01-11e8-abd7-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/a1a2f869-dd01-11e8-abd7-6c96cfdf3997",
							PrefLabel:  "Global Banking",
							Type:       "http://www.ft.com/ontology/organisation/Organisation",
							IsFTAuthor: false,
						},
					},
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/a1f96d3d-dd01-11e8-b32e-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/a1f96d3d-dd01-11e8-b32e-6c96cfdf3997",
							PrefLabel:  "Economy",
							Type:       "http://www.ft.com/ontology/organisation/Organisation",
							IsFTAuthor: false,
						},
					},
				},
				2: {
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/a2a0e506-dd01-11e8-b8b5-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/a2a0e506-dd01-11e8-b8b5-6c96cfdf3997",
							PrefLabel:  "Google Inc.",
							Type:       "http://www.ft.com/ontology/Topic",
							IsFTAuthor: false,
						},
					},
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/c517ae15-dd01-11e8-aaba-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/c517ae15-dd01-11e8-aaba-6c96cfdf3997",
							PrefLabel:  "Private company",
							Type:       "http://www.ft.com/ontology/Topic",
							IsFTAuthor: false,
						},
					},
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/c9e114c1-dd01-11e8-8d2b-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/c9e114c1-dd01-11e8-8d2b-6c96cfdf3997",
							PrefLabel:  "Company",
							Type:       "http://www.ft.com/ontology/Topic",
							IsFTAuthor: false,
						},
					},
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/ca26a873-dd01-11e8-9a17-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/ca26a873-dd01-11e8-9a17-6c96cfdf3997",
							PrefLabel:  "Apple",
							Type:       "http://www.ft.com/ontology/Topic",
							IsFTAuthor: false,
						},
					},
				},
			},
			expectedSuggestions: map[int][]Suggestion{
				1: {
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/a13db394-dd01-11e8-a00d-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/a13db394-dd01-11e8-a00d-6c96cfdf3997",
							PrefLabel:  "Bank of England",
							Type:       "http://www.ft.com/ontology/organisation/Organisation",
							IsFTAuthor: false,
						},
					},
				},
				2: {
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/a2a0e506-dd01-11e8-b8b5-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/a2a0e506-dd01-11e8-b8b5-6c96cfdf3997",
							PrefLabel:  "Google Inc.",
							Type:       "http://www.ft.com/ontology/Topic",
							IsFTAuthor: false,
						},
					},
					{
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/ca26a873-dd01-11e8-9a17-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/ca26a873-dd01-11e8-9a17-6c96cfdf3997",
							PrefLabel:  "Apple",
							Type:       "http://www.ft.com/ontology/Topic",
							IsFTAuthor: false,
						},
					},
				},
			},
		},
		{
			testName: "ok_noResultsFromPublicThings",
			publicThingsResponse: broaderResponse{
				Things: map[string]Thing{},
			},
			suggestions: map[int][]Suggestion{
				1: {
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
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							PrefLabel:  "Donald Kaberuka",
							Type:       "http://www.ft.com/ontology/person/Person",
							IsFTAuthor: false,
						},
					},
				},
			},
			expectedSuggestions: map[int][]Suggestion{
				1: {
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
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							PrefLabel:  "Donald Kaberuka",
							Type:       "http://www.ft.com/ontology/person/Person",
							IsFTAuthor: false,
						},
					},
				},
			},
		},
		{
			testName: "ok_errorFromPublicThings",
			publicThingsResponse: broaderResponse{
				Things: map[string]Thing{},
			},
			publicThingsError:     fmt.Errorf("error from publicThingsAPI"),
			expectedErrorContains: "error from publicThingsAPI",
			suggestions: map[int][]Suggestion{
				1: {
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
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							PrefLabel:  "Donald Kaberuka",
							Type:       "http://www.ft.com/ontology/person/Person",
							IsFTAuthor: false,
						},
					},
				},
			},
			expectedSuggestions: map[int][]Suggestion{
				1: {
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
						Predicate: "http://www.ft.com/ontology/annotation/mentions",
						Concept: Concept{
							ID:         "http://www.ft.com/thing/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							APIURL:     "http://api.ft.com/people/2d2657e2-dcff-11e8-a112-6c96cfdf3997",
							PrefLabel:  "Donald Kaberuka",
							Type:       "http://www.ft.com/ontology/person/Person",
							IsFTAuthor: false,
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		publicThingsRespBytes, err := json.Marshal(testCase.publicThingsResponse)
		ast.NoErrorf(err, "%s -> unexpected json marshal error", testCase.testName)

		publicThingsMock := new(mockHttpClient)
		if testCase.publicThingsStatusCode == 0 {
			testCase.publicThingsStatusCode = http.StatusOK
		}
		if !testCase.publicThingsCallSkipped {
			publicThingsMock.On("Do", mock.AnythingOfType("*http.Request")).Return(
				&http.Response{
					Body:       ioutil.NopCloser(bytes.NewReader(publicThingsRespBytes)),
					StatusCode: testCase.publicThingsStatusCode,
				},
				testCase.publicThingsError,
			).Once()
		}

		excludeService := NewBroaderConceptsProvider("dummyURL", "things", publicThingsMock)

		res, err := excludeService.excludeBroaderConceptsFromResponse(testCase.suggestions, "test_tid", "")
		if err != nil {
			ast.NotEmptyf(testCase.expectedErrorContains, "%s -> empty expected error", testCase.testName)
			ast.Containsf(err.Error(), testCase.expectedErrorContains, "%s -> not expected error returned", testCase.testName)
		} else {
			ast.NoErrorf(err, "%s -> unexpected error during excluding broader concepts", testCase.testName)
		}

		// this is in here because the BroaderConceptsProvider should return the same results as the ones that it received if an error occurrs
		ast.Lenf(res, len(testCase.expectedSuggestions), "%s -> unexpected results len", testCase.testName)
		for expectedIdx, expectedSourceResults := range testCase.expectedSuggestions {
			actualResults, ok := res[expectedIdx]
			ast.Truef(ok, "%s -> no source index %d found in the actual results", testCase.testName, expectedIdx)
			for _, expectedSuggestion := range expectedSourceResults {
				found := false
				for _, actualSuggestion := range actualResults {
					if actualSuggestion.ID == expectedSuggestion.ID {
						found = true
						break
					}
				}
				ast.Truef(found, "%s -> suggestion(ID: %s) not found in results", testCase.testName, expectedSuggestion.ID)
			}
		}
		publicThingsMock.AssertExpectations(t)
	}
}
