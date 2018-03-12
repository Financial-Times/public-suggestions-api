package main

import (
	"testing"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/golang/go/src/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewHealthServiceNoChecks(t *testing.T) {
	expect := assert.New(t)
	healthService := NewHealthService("test-system-code", "test-name", "test-description")

	expect.NotNil(healthService)
	expect.Equal("test-system-code", healthService.SystemCode)
	expect.Equal("test-name", healthService.Name)
	expect.Equal("test-description", healthService.Description)
	expect.Equal(10*time.Second, healthService.Timeout)
	expect.Nil(healthService.Checks)
	expect.Nil(healthService.gtgChecks)
}

func TestHealthService_GTGSuccessfully(t *testing.T) {
	expect := assert.New(t)

	check := fthealth.Check{Name: "test-check", BusinessImpact: "none", TechnicalSummary: "nothing", PanicGuide: "http://test-url.com", Severity: 2, Checker: func() (string, error) {
		return "everything-is-awesome", nil
	}}

	healthService := NewHealthService("", "", "", check)
	status := healthService.GTG()

	expect.Equal("", status.Message)
	expect.True(status.GoodToGo)
}

func TestHealthService_GTGError(t *testing.T) {
	expect := assert.New(t)

	check := fthealth.Check{Name: "test-check", BusinessImpact: "none", TechnicalSummary: "nothing", PanicGuide: "http://test-url.com", Severity: 2, Checker: func() (string, error) {
		return "", errors.New("everything-is-error")
	}}

	healthService := NewHealthService("", "", "", check)

	expect.NotNil(healthService.Checks)
	expect.True(len(healthService.Checks) == 1)
	expect.Equal("test-check", healthService.Checks[0].Name)
	expect.Equal("none", healthService.Checks[0].BusinessImpact)
	expect.Equal("nothing", healthService.Checks[0].TechnicalSummary)
	expect.Equal("http://test-url.com", healthService.Checks[0].PanicGuide)
	expect.Equal(uint8(2), healthService.Checks[0].Severity)

	status := healthService.GTG()

	expect.Equal("everything-is-error", status.Message)
	expect.False(status.GoodToGo)
}
