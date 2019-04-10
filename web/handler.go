package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/public-suggestions-api/service"
	tidutils "github.com/Financial-Times/transactionid-utils-go"
)

type RequestHandler struct {
	Suggester *service.AggregateSuggester
}

func NewRequestHandler(s *service.AggregateSuggester) *RequestHandler {
	return &RequestHandler{Suggester: s}
}

func (handler *RequestHandler) HandleSuggestion(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	tid := tidutils.GetTransactionIDFromRequest(request)

	debug := request.Header.Get("debug")

	body, err := ioutil.ReadAll(request.Body)
	if debug != "" {
		log.WithTransactionID(tid).WithField("debug", debug).Info(string(body))
	}
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

	suggestions, err := handler.Suggester.GetSuggestions(body, tid, service.Flags{Debug: debug})
	if err != nil {
		errMsg := "aggregating suggestions failed!"
		log.WithTransactionID(tid).WithError(err).Error(errMsg)
		writeResponse(writer, http.StatusServiceUnavailable, []byte(fmt.Sprintf(`{"message": "%v"}`, errMsg)))
		return
	}

	if len(suggestions.Suggestions) == 0 {
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
		return false, errors.New("valid but empty JSON request")
	}
	return true, nil
}

func writeResponse(writer http.ResponseWriter, status int, response []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(response)
}
