package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"testing"
)

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
			"9a5e3b4a-55da-498c-816f-9c534e1392b5": {
				ID:        "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392b5",
				APIURL:    "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392b5",
				PrefLabel: "Lawrence Summers",
				Type:      "http://www.ft.com/ontology/person/Person",
			},
			"9a5e3b4a-55da-498c-816f-9c534e139260": {
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
	queryParams.Add(idsParamName, "9a5e3b4a-55da-498c-816f-9c534e1392b5")
	queryParams.Add(idsParamName, "9a5e3b4a-55da-498c-816f-9c534e139265")
	queryParams.Add(idsParamName, "9a5e3b4a-55da-498c-816f-9c534e139260")
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
	suggestionResp.Suggestions = suggester.FilterSuggestions(suggestionResp.Suggestions, SourceFlags{Flags: defaultConceptsSources})

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
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": {
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": {
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": {
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
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
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": {
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": {
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": {
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				PersonSourceParam:       TmeSource,
				LocationSourceParam:     TmeSource,
				OrganisationSourceParam: TmeSource,
				TopicSourceParam:        TmeSource,
				PseudoConceptTypeAuthor: TmeSource,
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
				"f758ef56-c40a-3162-91aa-3e8a3aabc495": {
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a385": {
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a385",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a385",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a55": {
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				PersonSourceParam:       CesSource,
				LocationSourceParam:     CesSource,
				OrganisationSourceParam: CesSource,
				TopicSourceParam:        TmeSource,
				PseudoConceptTypeAuthor: TmeSource,
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
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": {
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a385": {
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a385",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a385",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": {
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				PersonSourceParam:       CesSource,
				LocationSourceParam:     TmeSource,
				OrganisationSourceParam: TmeSource,
				TopicSourceParam:        TmeSource,
				PseudoConceptTypeAuthor: TmeSource,
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
				"f758ef56-c40a-3162-91aa-3e8a3aabc495": {
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": {
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": {
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				PersonSourceParam:       TmeSource,
				LocationSourceParam:     CesSource,
				OrganisationSourceParam: TmeSource,
				TopicSourceParam:        TmeSource,
				PseudoConceptTypeAuthor: TmeSource,
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
				"f758ef56-c40a-3162-91aa-3e8a3aabc490": {
					ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc490",
					PrefLabel: "London",
					Type:      "http://www.ft.com/ontology/Location",
				},
				"64302452-e369-4ddb-88fa-9adc5124a380": {
					ID:        "http://www.ft.com/thing/64302452-e369-4ddb-88fa-9adc5124a380",
					APIURL:    "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a380",
					PrefLabel: "Eric Platt",
					Type:      "http://www.ft.com/ontology/person/Person",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a55": {
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					PrefLabel: "London Politics",
					Type:      "http://www.ft.com/ontology/Topic",
				},
			},
			flags: map[string]string{
				PersonSourceParam:       TmeSource,
				LocationSourceParam:     TmeSource,
				OrganisationSourceParam: CesSource,
				TopicSourceParam:        TmeSource,
				PseudoConceptTypeAuthor: TmeSource,
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
				"70121ea5-caf8-11e8-b0db-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3990",
					APIURL:    "http://api.ft.com/things/70121ea5-caf8-11e8-b0db-6c96cfdf3990",
					PrefLabel: "ABC",
					Type:      "http://www.ft.com/ontology/company/Company",
				},
				"ec0307f0-caf7-11e8-a9b5-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3990",
					APIURL:    "http://api.ft.com/things/ec0307f0-caf7-11e8-a9b5-6c96cfdf3990",
					PrefLabel: "Facebook",
					Type:      "http://www.ft.com/ontology/company/PrivateCompany",
				},
				"0767212c-caf9-11e8-b2ed-6c96cfdf3990": {
					ID:        "http://www.ft.com/thing/0767212c-caf9-11e8-b2ed-6c96cfdf3990",
					APIURL:    "http://api.ft.com/things/0767212c-caf9-11e8-b2ed-6c96cfdf3990",
					PrefLabel: "CIA",
					Type:      "http://www.ft.com/ontology/company/PublicCompany",
				},
				"9332270e-f959-3f55-9153-d30acd0d0a50": {
					ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a50",
					APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a50",
					PrefLabel: "Apple",
					Type:      "http://www.ft.com/ontology/organisation/Organisation",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
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
				LocationSourceParam:     TmeSource,
				PersonSourceParam:       TmeSource,
				OrganisationSourceParam: CesSource,
				TopicSourceParam:        TmeSource,
				PseudoConceptTypeAuthor: TmeSource,
			},
			expectedUUIDs: []string{
				"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				"http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
				"http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
			},
			internalConcordancesConcepts: map[string]Concept{
				"70121ea5-caf8-11e8-b0db-6c96cfdf3995": {
					ID:        "http://www.ft.com/thing/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
					APIURL:    "http://api.ft.com/things/70121ea5-caf8-11e8-b0db-6c96cfdf3995",
					PrefLabel: "ABC",
					Type:      "http://www.ft.com/ontology/company/Company",
				},
				"ec0307f0-caf7-11e8-a9b5-6c96cfdf3995": {
					ID:        "http://www.ft.com/thing/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
					APIURL:    "http://api.ft.com/things/ec0307f0-caf7-11e8-a9b5-6c96cfdf3995",
					PrefLabel: "Facebook",
					Type:      "http://www.ft.com/ontology/company/PrivateCompany",
				},
				"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
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

func TestAggregateSuggester_GetSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	suggestionApi := new(mockSuggestionApi)
	mockClient := new(mockHttpClient)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)

	falconSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
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
		{
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
	suggestionApi.On("FilterSuggestions", falconSuggestion.Suggestions, mock.Anything).Return(falconSuggestion.Suggestions).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", authorsSuggestion.Suggestions, mock.Anything).Return(authorsSuggestion.Suggestions).Once()

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
	suggestionApi.On("FilterSuggestions", falconSuggestion.Suggestions, mock.Anything).Return(falconSuggestion.Suggestions).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", authorsSuggestion.Suggestions, mock.Anything).Return(authorsSuggestion.Suggestions).Once()

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

	suggestionsResponse := SuggestionsResponse{Suggestions: []Suggestion{
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
	},
	}
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(suggestionsResponse, nil).Once()
	suggestionApi.On("FilterSuggestions", suggestionsResponse.Suggestions, mock.Anything).Return(suggestionsResponse.Suggestions).Once()

	defaultConceptsSources := buildDefaultConceptSources()
	aggregateSuggester := NewAggregateSuggester(mockConcordance, defaultConceptsSources, suggestionApi, suggestionApi)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", SourceFlags{Flags: defaultConceptsSources})

	expect.NoError(err)
	expect.Len(response.Suggestions, 1)

	expect.Equal(response.Suggestions[0].Concept.ID, "authors-suggestion-api")

	suggestionApi.AssertExpectations(t)
}
