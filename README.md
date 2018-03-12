# draft-suggestion-api

[![Circle CI](https://circleci.com/gh/Financial-Times/draft-suggestion-api/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/draft-suggestion-api/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/draft-suggestion-api)](https://goreportcard.com/report/github.com/Financial-Times/draft-suggestion-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/draft-suggestion-api/badge.svg)](https://coveralls.io/github/Financial-Times/draft-suggestion-api)

## Introduction

Service serving requests made towards suggestions umbrella

## Installation

Download the source code, dependencies and test dependencies:

        go get -u github.com/kardianos/govendor
        go get -u github.com/Financial-Times/draft-suggestion-api
        cd $GOPATH/src/github.com/Financial-Times/draft-suggestion-api
        govendor sync
        go build .

## Running locally

1. Run the tests and install the binary:

        govendor sync
        govendor test -v -race +local
        go install

2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/draft-suggestion-api [--help]

Options:

      --app-system-code                  System Code of the application (env $APP_SYSTEM_CODE) (default "draft-suggestion-api")
      --app-name                         Application name (env $APP_NAME) (default "draft-suggestion-api")
      --port                             Port to listen on (env $APP_PORT) (default "9090")
      --falcon-suggestion-api-base-url   The base URL to falcon suggestion api (env $FALCON_SUGGESTION_API_BASE_URL) (default "http://localhost:8080")
      --falcon-suggestion-endpoint       The endpoint for falcon suggestion api (env $FALCON_SUGGESTION_ENDPOINT) (default "/content/suggest/falcon")
        
3. Test:

    1. Either using curl:

            curl -d '{"test":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp

    1. Or using [httpie](https://github.com/jkbrzt/httpie):

            http POST http://localhost:8080/content/suggest test=content

## Build and deployment

* Built by Docker Hub on merge to master: [coco/draft-suggestion-api](https://hub.docker.com/r/coco/draft-suggestion-api/)
* CI provided by CircleCI: [draft-suggestion-api](https://circleci.com/gh/Financial-Times/draft-suggestion-api)

## Service endpoints

### POST

Using curl:

    curl -d '{"title":"tile", "byline": "byline", "bodyXML":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp

## Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

`/__build-info`

### Logging

* The application uses [logrus](https://github.com/Sirupsen/logrus); the log file is initialised in [main.go](main.go).