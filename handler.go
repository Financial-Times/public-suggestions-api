package main

import (
	"encoding/json"
	"net/http"
)

type requestHandler struct {
}

type suggestionsResponse struct {
	Suggestions []suggestionsType `json:"suggestions"`
}

func (handler *requestHandler) sampleMessage(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	aggregator := SuggestionsAggregator{}

	sampleResponse := &suggestionsResponse{
		aggregator.sampleMessage(),
	}

	sampleJsonResponse, err := json.Marshal(sampleResponse)

	switch err {
	case nil:
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(sampleJsonResponse)
	default:
		writer.WriteHeader(http.StatusInternalServerError)
	}

}
