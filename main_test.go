package main

import (
	"testing"
	"github.com/Financial-Times/draft-suggestion-api/web"
)

func Test_main(t *testing.T) {
	tests := []struct {
		name string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			main()
		})
	}
}

func Test_serveEndpoints(t *testing.T) {
	type args struct {
		appSystemCode  string
		appName        string
		port           string
		requestHandler *web.RequestHandler
	}
	tests := []struct {
		name string
		args args
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serveEndpoints(tt.args.appSystemCode, tt.args.appName, tt.args.port, tt.args.requestHandler)
		})
	}
}

func Test_waitForSignal(t *testing.T) {
	tests := []struct {
		name string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waitForSignal()
		})
	}
}
