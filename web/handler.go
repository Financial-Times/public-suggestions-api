package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/public-suggestions-api/reqorigin"
	"github.com/Financial-Times/public-suggestions-api/service"
	tidutils "github.com/Financial-Times/transactionid-utils-go"
)

type RequestHandler struct {
	suggester *service.AggregateSuggester
	log       *logger.UPPLogger
}

func NewRequestHandler(s *service.AggregateSuggester, log *logger.UPPLogger) *RequestHandler {
	return &RequestHandler{
		suggester: s,
		log:       log,
	}
}

func (h *RequestHandler) HandleSuggestion(resp http.ResponseWriter, req *http.Request) {

	tid := tidutils.GetTransactionIDFromRequest(req)
	logEntry := h.log.WithTransactionID(tid)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logEntry.WithError(err).Error("Error while reading payload")
		writeResponse(resp, http.StatusBadRequest, []byte(`{"message": "Error while reading payload"}`))
		return
	}

	logEntry.Debugf("request body: %s", string(body))
	validPayload, err := validatePayload(body)
	if !validPayload {
		logEntry.WithError(err).Error("Client error: payload should be a non-empty JSON object")
		writeResponse(resp, http.StatusBadRequest, []byte(`{"message": "Payload should be a non-empty JSON object"}`))
		return
	}

	suggestions, err := h.suggester.GetSuggestions(body, tid, reqorigin.FromRequest(req))
	if err != nil {
		errMsg := "aggregating suggestions failed!"
		logEntry.WithError(err).Error(errMsg)
		writeResponse(resp, http.StatusServiceUnavailable, []byte(fmt.Sprintf(`{"message": "%s"}`, errMsg)))
		return
	}

	if len(suggestions.Suggestions) == 0 {
		logEntry.Warn("Suggestions are empty")
	}
	//ignoring marshalling errors as neither UnsupportedTypeError nor UnsupportedValueError is possible
	jsonResponse, _ := json.Marshal(suggestions)

	writeResponse(resp, http.StatusOK, jsonResponse)
}

func validatePayload(content []byte) (bool, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(content, &payload); err != nil {
		return false, err
	}
	if len(payload) == 0 {
		return false, errors.New("valid but empty JSON request")
	}
	return true, nil
}

func writeResponse(writer http.ResponseWriter, status int, response []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(response)
}
