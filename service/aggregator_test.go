package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAggregateSuggester_GetAuthorSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	// create ontotext response mock
	ontotextMock := new(mockHttpClient)
	ontotextMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
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
	mockClientError := new(mockHttpClient)
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
	mockClientError.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: ioutil.NopCloser(strings.NewReader("")), StatusCode: http.StatusInternalServerError}, nil)

	// create all the services
	ontotextSuggester := NewOntotextSuggester("ontotextnUrl", "ontotextEndpoint", ontotextMock)
	authorsSuggester := NewAuthorsSuggester("authorsUrl", "authorsEndpoint", authorsMock)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientError)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, ontotextSuggester, authorsSuggester)

	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

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
	defer server.Close()

	suggester := NewOntotextSuggester(server.URL, "/content/suggest", http.DefaultClient)
	suggestionResp, err := suggester.GetSuggestions(body, "tid_test", Flags{})
	suggestionResp.Suggestions = suggester.FilterSuggestions(suggestionResp.Suggestions)

	actualSuggestions := suggestionResp.Suggestions
	expect.NoError(err)
	expect.NotNil(actualSuggestions)
	expect.Equal(1, len(actualSuggestions))

	for _, expected := range expectedSuggestions {
		expect.Contains(actualSuggestions, expected)
	}
	mock.AssertExpectationsForObjects(t, mockServer)
}


func TestAggregateSuggester_InternalConcordancesUnavailable(t *testing.T) {
	expect := assert.New(t)
	suggestionAPI := new(mockSuggestionApi)
	ontotextSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "ontotext-suggestion-api",
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
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(ontotextSuggestion, nil).Once()
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["ontotext-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "ontotext-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: ontologyPersonType,
	}

	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: ioutil.NopCloser(strings.NewReader(""))}, fmt.Errorf("error during calling internal concordances"))

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionAPI, suggestionAPI)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

	expect.Error(err)
	expect.Equal(err.Error(), "error during calling internal concordances")
	expect.Len(response.Suggestions, 0)

	suggestionAPI.AssertExpectations(t)
}

func TestAggregateSuggester_InternalConcordancesUnexpectedStatus(t *testing.T) {
	expect := assert.New(t)
	suggestionAPI := new(mockSuggestionApi)
	ontotextSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "ontotext-suggestion-api",
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
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(ontotextSuggestion, nil)
	suggestionAPI.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil)

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["ontotext-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "ontotext-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
	}
	mockInternalConcResp.Concepts["authors-suggestion-api"] = Concept{
		IsFTAuthor: true, ID: "authors-suggestion-api", APIURL: "apiurl2", PrefLabel: "prefLabel2", Type: ontologyPersonType,
	}

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	mockClient := new(mockHttpClient)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil).Once()
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil).Once()
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionAPI, suggestionAPI)

	// 503
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusServiceUnavailable,
	}, nil).Once()
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})
	expect.Error(err)
	expect.Equal("non 200 status code returned: 503", err.Error())
	expect.Len(response.Suggestions, 0)

	// 400
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusBadRequest,
	}, nil).Once()
	response, err = aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})
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

	ontotextSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "ontotext-suggestion-api",
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

	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(ontotextSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", ontotextSuggestion.Suggestions, mock.Anything).Return(ontotextSuggestion.Suggestions).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", authorsSuggestion.Suggestions, mock.Anything).Return(authorsSuggestion.Suggestions).Once()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["ontotext-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "ontotext-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
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

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionApi, suggestionApi)
	response, _ := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

	expect.Len(response.Suggestions, 2)

	expect.Contains(response.Suggestions, ontotextSuggestion.Suggestions[0])
	expect.Contains(response.Suggestions, authorsSuggestion.Suggestions[0])

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetPersonSuggestionsSuccessfully(t *testing.T) {
	expect := assert.New(t)
	suggestionApi := new(mockSuggestionApi)
	ontotextSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "ontotext-suggestion-api",
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
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(ontotextSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", ontotextSuggestion.Suggestions, mock.Anything).Return(ontotextSuggestion.Suggestions).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", authorsSuggestion.Suggestions, mock.Anything).Return(authorsSuggestion.Suggestions).Once()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["ontotext-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "ontotext-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
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

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)
	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionApi, suggestionApi)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

	expect.NoError(err)
	expect.Len(response.Suggestions, 2)

	expect.Contains(response.Suggestions, ontotextSuggestion.Suggestions[0])
	expect.Contains(response.Suggestions, authorsSuggestion.Suggestions[0])

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetEmptySuggestionsArrayIfNoAggregatedSuggestionAvailable(t *testing.T) {
	expect := assert.New(t)
	suggestionApi := new(mockSuggestionApi)
	mockConcordance := new(ConcordanceService)
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{}, errors.New("Ontotext err"))

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionApi, suggestionApi)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

	expect.NoError(err)
	expect.Len(response.Suggestions, 0)
	expect.NotNil(response.Suggestions)

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetSuggestionsNoErrorForOntotextSuggestionApi(t *testing.T) {
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

	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(SuggestionsResponse{}, errors.New("Ontotext err")).Once()

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

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":[]}`)),
		StatusCode: http.StatusOK,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionApi, suggestionApi)
	response, err := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

	expect.NoError(err)
	expect.Len(response.Suggestions, 1)

	expect.Equal(response.Suggestions[0].Concept.ID, "authors-suggestion-api")

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetSuggestionsWithBlacklist(t *testing.T) {
	expect := assert.New(t)

	suggestionApi := new(mockSuggestionApi)
	mockClient := new(mockHttpClient)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)

	ontotextSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "ontotext-suggestion-api",
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

	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(ontotextSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", ontotextSuggestion.Suggestions, mock.Anything).Return(ontotextSuggestion.Suggestions).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", authorsSuggestion.Suggestions, mock.Anything).Return(authorsSuggestion.Suggestions).Once()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["ontotext-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "ontotext-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
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

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"uuids":["ontotext-suggestion-api"]}`)),
		StatusCode: http.StatusOK,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionApi, suggestionApi)
	response, _ := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

	expect.Len(response.Suggestions, 1)

	expect.Contains(response.Suggestions, authorsSuggestion.Suggestions[0])

	suggestionApi.AssertExpectations(t)
}

func TestAggregateSuggester_GetSuggestionsWithBlacklistError(t *testing.T) {
	expect := assert.New(t)

	suggestionApi := new(mockSuggestionApi)
	mockClient := new(mockHttpClient)
	mockConcordance := NewConcordance("internalConcordancesHost", "/internalconcordances", mockClient)

	ontotextSuggestion := SuggestionsResponse{Suggestions: []Suggestion{
		{
			Predicate: "predicate",
			Concept: Concept{
				IsFTAuthor: false,
				ID:         "ontotext-suggestion-api",
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

	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(ontotextSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", ontotextSuggestion.Suggestions, mock.Anything).Return(ontotextSuggestion.Suggestions).Once()
	suggestionApi.On("GetSuggestions", mock.AnythingOfType("[]uint8"), "tid_test").Return(authorsSuggestion, nil).Once()
	suggestionApi.On("FilterSuggestions", authorsSuggestion.Suggestions, mock.Anything).Return(authorsSuggestion.Suggestions).Once()

	mockInternalConcResp := ConcordanceResponse{
		Concepts: make(map[string]Concept),
	}
	mockInternalConcResp.Concepts["ontotext-suggestion-api"] = Concept{
		IsFTAuthor: false, ID: "ontotext-suggestion-api", APIURL: "apiurl1", PrefLabel: "prefLabel1", Type: ontologyPersonType,
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

	mockClientPublicThings := new(mockHttpClient)
	mockClientPublicThings.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: http.StatusOK,
	}, nil)

	broaderProvider := NewBroaderConceptsProvider("publicThingsUrl", "/things", mockClientPublicThings)

	blacklisterMock := new(mockHttpClient)
	blacklisterMock.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		Body: ioutil.NopCloser(strings.NewReader(
			`{"message":"server error"}`)),
		StatusCode: http.StatusInternalServerError,
	}, nil)
	blacklister := NewConceptBlacklister("blacklisterUrl", "blacklisterEndpoint", blacklisterMock)

	aggregateSuggester := NewAggregateSuggester(mockConcordance, broaderProvider, blacklister, suggestionApi, suggestionApi)
	response, _ := aggregateSuggester.GetSuggestions([]byte{}, "tid_test", Flags{})

	expect.Len(response.Suggestions, 2)

	expect.Contains(response.Suggestions, ontotextSuggestion.Suggestions[0])
	expect.Contains(response.Suggestions, authorsSuggestion.Suggestions[0])

	suggestionApi.AssertExpectations(t)
}
