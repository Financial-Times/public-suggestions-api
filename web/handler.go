package web

import (
	"encoding/json"
	"net/http"
	"github.com/Financial-Times/draft-suggestion-api/suggestion"
	"io/ioutil"
)

type RequestHandler struct {
	Aggregator suggestion.Aggregator
}

type suggestionsResponse struct {
	Suggestions []suggestion.Response `json:"suggestions"`
}

func NewRequestHandler(aggr suggestion.Aggregator) *RequestHandler {
	return &RequestHandler{Aggregator: aggr}
}

func (handler *RequestHandler) HandleSuggestion(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		writeResponse(writer, http.StatusBadRequest, []byte(`{"message": "Error by reading payload"}`))
		return

	}

	if len(body) == 0 {
		writeResponse(writer, http.StatusBadRequest, []byte(`{"message": "Payload should not be empty"}`))
		return
	}
	// TODO add some validation for required fields
	response := &suggestionsResponse{
		handler.Aggregator.HandleSuggestion(body),
	}

	jsonResponse, err := json.Marshal(response)

	switch err {
	case nil:
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(jsonResponse)
	default:
		writer.WriteHeader(http.StatusInternalServerError)
	}

}
func writeResponse(writer http.ResponseWriter, status int, response []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(response)
}
