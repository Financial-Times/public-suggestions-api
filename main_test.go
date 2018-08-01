package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	log "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/public-suggestions-api/service"
	"github.com/Financial-Times/public-suggestions-api/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"net/http/httptest"
	"strings"
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
	case _ = <-waitCh:
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

	expectedSuggestions := []service.Suggestion{
		{
			Predicate:      "http://www.ft.com/ontology/annotation/mentions",
			Id:             "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
			ApiUrl:         "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
			PrefLabel:      "Donald Kaberuka",
			SuggestionType: "http://www.ft.com/ontology/person/Person",
			IsFTAuthor:     false,
		},
		{
			Predicate:      "http://www.ft.com/ontology/annotation/hasAuthor",
			Id:             "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
			ApiUrl:         "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
			PrefLabel:      "Lawrence Summers",
			SuggestionType: "http://www.ft.com/ontology/person/Person",
			IsFTAuthor:     true,
		},
	}
	tests := []struct {
		url                 string
		expectedStatus      int
		expectedSuggestions []service.Suggestion
	}{
		{url: "http://localhost:8080/content/suggest?source=tme&source=authors", expectedStatus: http.StatusOK, expectedSuggestions: expectedSuggestions},
		{url: "http://localhost:8080/content/suggest?source=tme", expectedStatus: http.StatusOK, expectedSuggestions: []service.Suggestion{
			expectedSuggestions[0],
			expectedSuggestions[1],
		}},
		{url: "http://localhost:8080/content/suggest?source=authors", expectedStatus: http.StatusOK, expectedSuggestions: []service.Suggestion{
			expectedSuggestions[1],
		}},
		{url: "http://localhost:8080/content/suggest", expectedStatus: http.StatusOK, expectedSuggestions: []service.Suggestion{
			expectedSuggestions[0],
			expectedSuggestions[1],
		}},
	}

	log.InitDefaultLogger("test")
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		w.WriteHeader(status)
		switch {
		case strings.Contains(r.RequestURI, "/falcon"):
			w.Write([]byte(`{
    		"suggestions": [{
    		        "predicate": "http://www.ft.com/ontology/annotation/mentions",
    		        "id": "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
    		        "apiUrl": "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
    		        "prefLabel": "Donald Kaberuka",
    		        "type": "http://www.ft.com/ontology/person/Person",
    		        "isFTAuthor": false
    		    },
    		    {
    		        "predicate": "http://www.ft.com/ontology/annotation/hasAuthor",
    		        "id": "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
    		        "apiUrl": "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
    		        "prefLabel": "Lawrence Summers",
    		        "type": "http://www.ft.com/ontology/person/Person",
    		        "isFTAuthor": true
    		    }
    		]}`))

		case strings.Contains(r.RequestURI, "/authors"):
			w.Write([]byte(`{
    		"suggestions": [
    		    {
    		        "predicate": "http://www.ft.com/ontology/annotation/hasAuthor",
    		        "id": "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
    		        "apiUrl": "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
    		        "prefLabel": "Lawrence Summers",
    		        "type": "http://www.ft.com/ontology/person/Person",
    		        "isFTAuthor": true
    		    }
    		]}`))
		}
	}))

	tr := &http.Transport{
		MaxIdleConnsPerHost: 128,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
	}
	c := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	falconSuggester := service.NewFalconSuggester(mockServer.URL, "/falcon", c)
	authorsSuggester := service.NewAuthorsSuggester(mockServer.URL, "/authors", c)
	suggester := service.NewAggregateSuggester(falconSuggester, authorsSuggester)
	healthService := NewHealthService("mock", "mock", "", falconSuggester.Check(), authorsSuggester.Check())

	go func() {
		serveEndpoints("8080", web.NewRequestHandler(suggester), healthService)
	}()

	client := &http.Client{}

	for _, test := range tests {

		req, _ := http.NewRequest("POST", test.url, strings.NewReader(`{"body":"test"}`))
		res, err := client.Do(req)
		assert.NoError(t, err)

		assert.Equal(t, test.expectedStatus, res.StatusCode)
		if test.expectedStatus == http.StatusOK {
			rBody := make([]byte, res.ContentLength)
			res.Body.Read(rBody)
			res.Body.Close()

			suggestionsResponse := service.SuggestionsResponse{}
			json.Unmarshal(rBody, &suggestionsResponse)
			assert.Equal(t, test.expectedSuggestions, suggestionsResponse.Suggestions)
		}
	}

}
