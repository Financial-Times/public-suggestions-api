package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

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
