package main

import (
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/Financial-Times/go-logger"
	"github.com/jawher/mow.cli"

	"github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/gorilla/mux"
	"github.com/rcrowley/go-metrics"

	"net"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/public-suggestions-api/service"
	"github.com/Financial-Times/public-suggestions-api/web"
	status "github.com/Financial-Times/service-status-go/httphandlers"
)

const appDescription = "Service serving requests made towards suggestions umbrella"
const suggestPath = "/content/suggest"

func main() {
	app := cli.App("public-suggestions-api", appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "public-suggestions-api",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})
	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "public-suggestions-api",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})
	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})
	falconSuggestionApiBaseURL := app.String(cli.StringOpt{
		Name:   "falcon-suggestion-api-base-url",
		Value:  "http://falcon-suggestion-api:8080",
		Desc:   "The base URL to falcon suggestion api",
		EnvVar: "FALCON_SUGGESTION_API_BASE_URL",
	})
	falconSuggestionEndpoint := app.String(cli.StringOpt{
		Name:   "falcon-suggestion-endpoint",
		Value:  "/content/suggest/falcon",
		Desc:   "The endpoint for falcon suggestion api",
		EnvVar: "FALCON_SUGGESTION_ENDPOINT",
	})
	authorsSuggestionApiBaseURL := app.String(cli.StringOpt{
		Name:   "authors-suggestion-api-base-url",
		Value:  "http://authors-suggestion-api:8080",
		Desc:   "The base URL to authors suggestion api",
		EnvVar: "AUTHORS_SUGGESTION_API_BASE_URL",
	})
	authorsSuggestionEndpoint := app.String(cli.StringOpt{
		Name:   "authors-suggestion-endpoint",
		Value:  "/content/suggest/authors",
		Desc:   "The endpoint for authors suggestion api",
		EnvVar: "AUTHORS_SUGGESTION_ENDPOINT",
	})
	CESBaseURL := app.String(cli.StringOpt{
		Name:   "ces-api-base-url",
		Value:  "http://ces:8080",
		Desc:   "The base URL to CES",
		EnvVar: "CES_BASE_URL",
	})
	CESEndpoint := app.String(cli.StringOpt{
		Name:   "ces-endpoint",
		Value:  "/content/suggest/",
		Desc:   "The endpoint for people and orgs suggestion api",
		EnvVar: "CES_ENDPOINT",
	})

	log.InitDefaultLogger(*appName)
	log.Infof("[Startup] public-suggestions-api is starting")

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)

		tr := &http.Transport{
			MaxIdleConnsPerHost: 128,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
		}
		c := &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		}
		falconSuggester := service.NewFalconSuggester(*falconSuggestionApiBaseURL, *falconSuggestionEndpoint, c)
		authorsSuggester := service.NewAuthorsSuggester(*authorsSuggestionApiBaseURL, *authorsSuggestionEndpoint, c)
		peopleAndOrgsSuggester := service.NewPeopleAndOrgsSuggester(*CESBaseURL, *CESEndpoint, c)
		suggester := service.NewAggregateSuggester(falconSuggester, authorsSuggester, peopleAndOrgsSuggester)
		healthService := NewHealthService(*appSystemCode, *appName, appDescription, falconSuggester.Check(), authorsSuggester.Check(), peopleAndOrgsSuggester.Check())

		serveEndpoints(*port, web.NewRequestHandler(suggester), healthService)

	}
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func serveEndpoints(port string, handler *web.RequestHandler, healthService *HealthService) {

	serveMux := http.NewServeMux()

	serveMux.HandleFunc(healthPath, fthealth.Handler(healthService))
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc(suggestPath, handler.HandleSuggestion).Methods(http.MethodPost)

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.Logger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	serveMux.Handle("/", monitoringRouter)

	server := &http.Server{Addr: ":" + port, Handler: serveMux}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Infof("HTTP server closing with message: %v", err)
		}
		wg.Done()
	}()

	waitForSignal()
	log.Infof("[Shutdown] public-suggestions-api is shutting down")

	if err := server.Close(); err != nil {
		log.Errorf("Unable to stop http server: %v", err)
	}

	wg.Wait()
}

func waitForSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
