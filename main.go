package main

import (
	"github.com/jawher/mow.cli"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/gorilla/mux"
	"github.com/rcrowley/go-metrics"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Financial-Times/draft-suggestion-api/web"
	"github.com/Financial-Times/draft-suggestion-api/suggestion"
)

const appDescription = "Service serving requests made towards suggestions umbrella"
const suggestPath = "/content/suggest"

func main() {
	app := cli.App("draft-suggestion-api", appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "draft-suggestion-api",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})

	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "draft-suggestion-api",
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

	log.SetLevel(log.InfoLevel)
	log.Infof("[Startup] draft-suggestion-api is starting")

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)

		aggregator := suggestion.NewAggregator(*falconSuggestionApiBaseURL)

		go func() {
			serveEndpoints(*appSystemCode, *appName, *port, web.NewRequestHandler(aggregator))
		}()

		waitForSignal()
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func serveEndpoints(appSystemCode, appName, port string, handler *web.RequestHandler) {
	healthService := newHealthService(appSystemCode, appName, appDescription)

	serveMux := http.NewServeMux()

	serveMux.HandleFunc(healthPath, http.HandlerFunc(fthealth.Handler(healthService.Health())))
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc(suggestPath, handler.HandleSuggestion).Methods(http.MethodPost)

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
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
	log.Infof("[Shutdown] draft-suggestion-api is shutting down")

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
