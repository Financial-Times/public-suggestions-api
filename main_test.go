package main

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/public-suggestions-api/service"
	"github.com/Financial-Times/public-suggestions-api/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainApp(t *testing.T) {
	expect := require.New(t)

	testCases := []struct {
		endpoint       string
		assertResponse func(response *http.Response)
	}{
		{
			"/__gtg",
			func(resp *http.Response) {
				defer resp.Body.Close()
				expect.Equal(http.StatusOK, resp.StatusCode)
			},
		},
		{
			"/__health",
			func(resp *http.Response) {
				defer resp.Body.Close()
				expect.Equal(http.StatusOK, resp.StatusCode)
				body, err := ioutil.ReadAll(resp.Body)
				expect.NoError(err)
				var checkResult map[string]interface{}
				err = json.Unmarshal(body, &checkResult)
				expect.NoError(err)
				systemCode, found := checkResult["systemCode"] //check there is a valid response
				expect.True(found)
				expect.Equal("public-suggestions-api", systemCode.(string))
			},
		},
	}

	waitCh := make(chan struct{})
	go func() {
		os.Args = []string{"public-suggestions-api"}
		main()
		waitCh <- struct{}{}
	}()
	select {
	case <-waitCh:
		expect.FailNow("Main should be running")
	case <-time.After(3 * time.Second):
		for _, testCase := range testCases {
			resp, err := http.Get("http://localhost:8080" + testCase.endpoint)
			expect.NoError(err)
			expect.NotNil(resp)
			testCase.assertResponse(resp)
		}
	}
}

func TestRequestHandler_all(t *testing.T) {
	expectedAuthorsSuggestions := []service.Suggestion{
		{
			Predicate: "http://www.ft.com/ontology/annotation/hasAuthor",
			Concept: service.Concept{
				ID:         "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392b6",
				APIURL:     "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392b6",
				PrefLabel:  "Lawrence Summers",
				Type:       "http://www.ft.com/ontology/person/Person",
				IsFTAuthor: true,
			},
		},
	}

	expectedOntotextSuggestions := []service.Suggestion{
		{
			Concept: service.Concept{
				ID:        "http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				APIURL:    "http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
				PrefLabel: "London Politics",
				Type:      "http://www.ft.com/ontology/Topic",
			},
		},
		{
			Concept: service.Concept{
				ID:        "http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
				APIURL:    "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
				PrefLabel: "London",
				Type:      "http://www.ft.com/ontology/Location",
			},
		},
		{
			Concept: service.Concept{
				ID:        "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c7355",
				APIURL:    "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c7355",
				PrefLabel: "Donald Kaberuka",
				Type:      "http://www.ft.com/ontology/person/Person",
			},
		},
		{
			Concept: service.Concept{
				ID:        "http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
				APIURL:    "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
				PrefLabel: "Apple",
				Type:      "http://www.ft.com/ontology/organisation/Organisation",
			},
		},
	}
	tests := []struct {
		testName            string
		url                 string
		expectedStatus      int
		expectedSuggestions []service.Suggestion
	}{
		{
			testName:       "okSuggestions",
			url:            "http://localhost:8081/content/suggest",
			expectedStatus: http.StatusOK,
			expectedSuggestions: []service.Suggestion{
				expectedOntotextSuggestions[2],
				expectedOntotextSuggestions[0],
				expectedOntotextSuggestions[3],
				expectedAuthorsSuggestions[0],
				expectedOntotextSuggestions[1],
			},
		},
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		w.WriteHeader(status)
		switch {
		case strings.Contains(r.RequestURI, "/authors"):
			_, _ = w.Write([]byte(`{
				"suggestions":[
				  {
					"predicate":"http://www.ft.com/ontology/annotation/hasAuthor",
					"id":"http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392b6",
					"apiUrl":"http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392b6",
					"prefLabel":"Lawrence Summers",
					"type":"http://www.ft.com/ontology/person/Person",
					"isFTAuthor":true
				  }
				]
			  }`))
		case strings.Contains(r.RequestURI, "/ontotext"):
			_, _ = w.Write([]byte(`{
				"suggestions":[
				  {
					"id":"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					"apiUrl":"http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
					"prefLabel":"London Politics",
					"type":"http://www.ft.com/ontology/Topic"
				  },
				  {
					"id":"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					"apiUrl":"http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
					"prefLabel":"London",
					"type":"http://www.ft.com/ontology/Location"
				  },
				  {
					"id":"http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c7355",
					"apiUrl":"http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c7355",
					"prefLabel":"Donald Kaberuka",
					"type":"http://www.ft.com/ontology/person/Person"
				  },
				  {
					"id":"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
					"apiUrl":"http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
					"prefLabel":"Apple",
					"type":"http://www.ft.com/ontology/organisation/Organisation"
				  }
				]
			  }`))
		case strings.Contains(r.RequestURI, "/internalconcordances"):
			_, _ = w.Write([]byte(`{
					"concepts": {
						"6f14ea94-690f-3ed4-98c7-b926683c7355": {
							"id": "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c7355",
							"apiUrl": "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c7355",
							"type": "http://www.ft.com/ontology/person/Person",
							"prefLabel": "Donald Kaberuka",
							"isFTAuthor": false
						},
						"7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990": {
							"id":"http://www.ft.com/thing/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
							"apiUrl":"http://api.ft.com/people/7e78cb61-c6f6-11e8-8ddc-6c96cfdf3990",
							"prefLabel":"London Politics",
							"type":"http://www.ft.com/ontology/Topic"
						},
						"f758ef56-c40a-3162-91aa-3e8a3aabc495": {
							"id":"http://www.ft.com/thing/f758ef56-c40a-3162-91aa-3e8a3aabc495",
							"apiUrl":"http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc495",
							"prefLabel":"London",
							"type":"http://www.ft.com/ontology/Location"
						},
						"9332270e-f959-3f55-9153-d30acd0d0a55": {
							"id":"http://www.ft.com/thing/9332270e-f959-3f55-9153-d30acd0d0a55",
							"apiUrl":"http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a55",
							"prefLabel":"Apple",
							"type":"http://www.ft.com/ontology/organisation/Organisation"
						},
						"9a5e3b4a-55da-498c-816f-9c534e1392b6": {
							"predicate":"http://www.ft.com/ontology/annotation/hasAuthor",
							"id":"http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392b6",
							"apiUrl":"http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392b6",
							"prefLabel":"Lawrence Summers",
							"type":"http://www.ft.com/ontology/person/Person",
							"isFTAuthor":true
						}
					}
				}`))
		case strings.Contains(r.RequestURI, "/things"):
			_, _ = w.Write([]byte(`{
					"things": {}
				}`))
		case strings.Contains(r.RequestURI, "/blacklist"):
			_, _ = w.Write([]byte(`{
					"uuids": []
				}`))
		}
	}))

	tr := &http.Transport{
		MaxIdleConnsPerHost: 128,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	c := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	log := logger.NewUPPLogger("test-service", "panic")
	authorsSuggester := service.NewAuthorsSuggester(mockServer.URL, "/authors", c)
	ontotextSuggester := service.NewOntotextSuggester(mockServer.URL, "/ontotext", c)
	concordance := service.NewConcordance(mockServer.URL, "/internalconcordances", c)
	broaderProvider := service.NewBroaderConceptsProvider(mockServer.URL, "/things", c)
	blacklister := service.NewCachedConceptFilter(mockServer.URL, "/blacklist", c)

	suggester := service.NewAggregateSuggester(log, concordance, broaderProvider, blacklister, authorsSuggester, ontotextSuggester)
	healthService := web.NewHealthService("mock", "mock", "", authorsSuggester.Check(), ontotextSuggester.Check(), broaderProvider.Check())

	go func() {
		serveEndpoints("8081", web.NewRequestHandler(suggester, log), healthService, log)
	}()
	client := &http.Client{}

	for _, test := range tests {

		req, _ := http.NewRequest("POST", test.url, strings.NewReader(`{"body":"test"}`))
		res, err := client.Do(req)
		assert.NoErrorf(t, err, "%s -> unexpected error", test.testName)

		assert.Equalf(t, test.expectedStatus, res.StatusCode, "%s -> unexpected status code", test.testName)
		if test.expectedStatus == http.StatusOK {
			rBody := make([]byte, res.ContentLength)
			_, _ = res.Body.Read(rBody)
			res.Body.Close()

			suggestionsResponse := service.SuggestionsResponse{}
			_ = json.Unmarshal(rBody, &suggestionsResponse)
			suggestions := suggestionsResponse.Suggestions
			sort.Slice(suggestions, func(i, j int) bool {
				return suggestions[i].Concept.ID < suggestions[j].Concept.ID
			})
			assert.Equalf(t, test.expectedSuggestions, suggestionsResponse.Suggestions, "%s -> not the same suggestions", test.testName)
		}
	}

}
