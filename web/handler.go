package web

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/Financial-Times/draft-suggestion-api/service"
	tidutils "github.com/Financial-Times/transactionid-utils-go"

	log "github.com/Financial-Times/go-logger"
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
		log.WithTransactionID(tid).WithError(err).Error("Error by reading payload")
		writeResponse(writer, http.StatusBadRequest, []byte(`{"message": "Error by reading payload"}`))
		return
	}

	if len(body) == 0 {
		log.WithTransactionID(tid).Error("Client error: payload should not be empty")
		writeResponse(writer, http.StatusBadRequest, []byte(`{"message": "Payload should not be empty"}`))
		return
	}

	suggestions, err := handler.Suggester.GetSuggestions(body, tid)
	if err != nil {
		log.WithTransactionID(tid).WithField("tid", tid).WithError(err).Error("Error calling Falcon suggestions API")
		writeResponse(writer, http.StatusServiceUnavailable, []byte(`{"message": "Requesting suggestions failed. Please try again"}`))
		return
	}
	if len(suggestions.Suggestions) == 0 {
		log.WithTransactionID(tid).Warn("Suggestions are empty")
	}
	//ignoring marshalling errors as neither UnsupportedTypeError nor UnsupportedValueError is possible
	jsonResponse, _ := json.Marshal(&suggestions)

	writeResponse(writer, http.StatusOK, jsonResponse)

}
func writeResponse(writer http.ResponseWriter, status int, response []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(response)
}
