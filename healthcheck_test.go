package main

import (
	"reflect"
	"testing"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

func Test_newHealthService(t *testing.T) {
	type args struct {
		appSystemCode  string
		appName        string
		appDescription string
	}
	tests := []struct {
		name string
		args args
		want *HealthService
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newHealthService(tt.args.appSystemCode, tt.args.appName, tt.args.appDescription); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newHealthService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthService_Health(t *testing.T) {
	type fields struct {
		config       *HealthConfig
		healthChecks []fthealth.Check
		gtgChecks    []gtg.StatusChecker
	}
	tests := []struct {
		name   string
		fields fields
		want   fthealth.HC
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &HealthService{
				config:       tt.fields.config,
				healthChecks: tt.fields.healthChecks,
				gtgChecks:    tt.fields.gtgChecks,
			}
			if got := service.Health(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HealthService.Health() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthService_sampleCheck(t *testing.T) {
	type fields struct {
		config       *HealthConfig
		healthChecks []fthealth.Check
		gtgChecks    []gtg.StatusChecker
	}
	tests := []struct {
		name   string
		fields fields
		want   fthealth.Check
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &HealthService{
				config:       tt.fields.config,
				healthChecks: tt.fields.healthChecks,
				gtgChecks:    tt.fields.gtgChecks,
			}
			if got := service.sampleCheck(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HealthService.sampleCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthService_sampleChecker(t *testing.T) {
	type fields struct {
		config       *HealthConfig
		healthChecks []fthealth.Check
		gtgChecks    []gtg.StatusChecker
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &HealthService{
				config:       tt.fields.config,
				healthChecks: tt.fields.healthChecks,
				gtgChecks:    tt.fields.gtgChecks,
			}
			got, err := service.sampleChecker()
			if (err != nil) != tt.wantErr {
				t.Errorf("HealthService.sampleChecker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HealthService.sampleChecker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_gtgCheck(t *testing.T) {
	type args struct {
		handler func() (string, error)
	}
	tests := []struct {
		name string
		args args
		want gtg.Status
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gtgCheck(tt.args.handler); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("gtgCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthService_GTG(t *testing.T) {
	type fields struct {
		config       *HealthConfig
		healthChecks []fthealth.Check
		gtgChecks    []gtg.StatusChecker
	}
	tests := []struct {
		name   string
		fields fields
		want   gtg.Status
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &HealthService{
				config:       tt.fields.config,
				healthChecks: tt.fields.healthChecks,
				gtgChecks:    tt.fields.gtgChecks,
			}
			if got := service.GTG(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HealthService.GTG() = %v, want %v", got, tt.want)
			}
		})
	}
}
