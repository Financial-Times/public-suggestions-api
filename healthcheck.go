package main

import (
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

const healthPath = "/__health"

type HealthService struct {
	fthealth.TimedHealthCheck
	gtgChecks []gtg.StatusChecker
}

func NewHealthService(appSystemCode string, appName string, appDescription string, checks ...fthealth.Check) *HealthService {
	var gtgChecks []gtg.StatusChecker
	for _, ch := range checks {
		gtgChecks = append(gtgChecks, func() gtg.Status {
			return gtgCheck(ch.Checker)
		})

	}
	return &HealthService{
		TimedHealthCheck: fthealth.TimedHealthCheck{
			HealthCheck: fthealth.HealthCheck{
				SystemCode:  appSystemCode,
				Name:        appName,
				Description: appDescription,
				Checks:      checks,
			},
			Timeout: 10 * time.Second,
		},
		gtgChecks: gtgChecks,
	}

}

func gtgCheck(handler func() (string, error)) gtg.Status {
	if _, err := handler(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}
	return gtg.Status{GoodToGo: true}
}

func (service *HealthService) GTG() gtg.Status {
	return gtg.FailFastParallelCheck(service.gtgChecks)()
}
