package web

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/Financial-Times/draft-suggestion-api/service"
	tidutils "github.com/Financial-Times/transactionid-utils-go"

	log "github.com/Financial-Times/go-logger"
	"errors"
)

type RequestHandler struct {
	Suggester service.Suggester
}

func NewRequestHandler(s service.Suggester) *RequestHandler {
	return &RequestHandler{Suggester: s}
}

func (handler *RequestHandler) HandleSuggestion(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	tid := tidutils.GetTransactionIDFromRequest(request)

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.WithTransactionID(tid).WithError(err).Error("Error while reading payload")
		writeResponse(writer, http.StatusBadRequest, []byte(`{"message": "Error while reading payload"}`))
		return
	}
	validPayload, err := validatePayload(body)
	if !validPayload {
		log.WithTransactionID(tid).WithError(err).Error("Client error: payload should be a non-empty JSON object")
		writeResponse(writer, http.StatusBadRequest, []byte(`{"message": "Payload should be a non-empty JSON object"}`))
		return
	}

	suggestions, err := handler.Suggester.GetSuggestions(body, tid)
	if err != nil {
		if err == service.NoContentError {
			log.WithTransactionID(tid).WithField("tid", tid).Warn(err.Error())
		} else {
			log.WithTransactionID(tid).WithField("tid", tid).WithError(err).Error("Error calling Falcon Suggestion API")
			writeResponse(writer, http.StatusServiceUnavailable, []byte(`{"message": "Requesting suggestions failed"}`))
			return
		}
	}
	if len(suggestions.Suggestions) == 0 && err == nil {
		log.WithTransactionID(tid).Warn("Suggestions are empty")
	}
	//ignoring marshalling errors as neither UnsupportedTypeError nor UnsupportedValueError is possible
	jsonResponse, _ := json.Marshal(suggestions)

	writeResponse(writer, http.StatusOK, jsonResponse)

}

func validatePayload(content []byte) (bool, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(content, &payload); err != nil {
		return false, err
	}
	if len(payload) == 0 {
		return false, errors.New("Valid but empty JSON request")
	}
	return true, nil
}

func writeResponse(writer http.ResponseWriter, status int, response []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(response)
}
