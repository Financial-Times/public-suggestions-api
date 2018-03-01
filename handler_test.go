package main

import (
	"net/http"
	"testing"
)

func Test_requestHandler_sampleMessage(t *testing.T) {
	type args struct {
		writer  http.ResponseWriter
		request *http.Request
	}
	tests := []struct {
		name    string
		handler *requestHandler
		args    args
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &requestHandler{}
			handler.sampleMessage(tt.args.writer, tt.args.request)
		})
	}
}
