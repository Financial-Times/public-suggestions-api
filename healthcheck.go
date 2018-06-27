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
		gtgChecks: []gtg.StatusChecker{
			func() gtg.Status {
				// always return as gtg, since we don't want to block overall suggestions if one of the downstream services are not working
				return gtg.Status{GoodToGo: true}
			},
		},
	}

}

func (service *HealthService) GTG() gtg.Status {
	return gtg.FailFastParallelCheck(service.gtgChecks)()
}
