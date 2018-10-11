# public-suggestions-api

[![Circle CI](https://circleci.com/gh/Financial-Times/public-suggestions-api/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/public-suggestions-api/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/public-suggestions-api)](https://goreportcard.com/report/github.com/Financial-Times/public-suggestions-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/public-suggestions-api/badge.svg)](https://coveralls.io/github/Financial-Times/public-suggestions-api)

## Introduction

Service serving requests made towards suggestions umbrella

## Installation

Download the source code, dependencies and test dependencies:

        curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
        go get -u github.com/Financial-Times/public-suggestions-api
        cd $GOPATH/src/github.com/Financial-Times/public-suggestions-api
        dep ensure -v -vendor-only
        go build .

## Running locally

1. Run the tests and install the binary:

        dep ensure -v
        go test -v -race ./...
        go install

2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/public-suggestions-api [--help]

Options:

      --app-system-code                    System Code of the application (env $APP_SYSTEM_CODE) (default "public-suggestions-api")
      --app-name                           Application name (env $APP_NAME) (default "public-suggestions-api")
      --port                               Port to listen on (env $APP_PORT) (default "9090")
      --falcon-suggestion-api-base-url     The base URL to falcon suggestion api (env $FALCON_SUGGESTION_API_BASE_URL) (default "http://localhost:8080")
      --falcon-suggestion-endpoint         The endpoint for falcon suggestion api (env $FALCON_SUGGESTION_ENDPOINT) (default "/content/suggest/falcon")
      --authors-suggestion-api-base-url    The base URL to authors suggestion api (env $AUTHORS_SUGGESTION_API_BASE_URL) (default "http://authors-suggestion-api:8080")
      --authors-suggestion-endpoint        The endpoint for authors suggestion api (env $AUTHORS_SUGGESTION_ENDPOINT) (default "/content/suggest/authors")
      --internal-concordances-api-base-url The base URL for internal concept concordances api (env $CONCEPT_CONCORDANCES_API_BASE_URL) (default "http://internal-concordances:8080")
      --internal-concordances-endpoint     The endpoint for internal concept concordances api (CONCEPT_CONCORDANCES_ENDPOINT) (default "/internalconcordances")
3. Test:

    1. Either using curl:

            curl -d '{"test":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp

    1. Or using [httpie](https://github.com/jkbrzt/httpie):

            http POST http://localhost:8080/content/suggest test=content

## Build and deployment

* Built by Docker Hub on merge to master: [coco/public-suggestions-api](https://hub.docker.com/r/coco/public-suggestions-api/)
* CI provided by CircleCI: [public-suggestions-api](https://circleci.com/gh/Financial-Times/public-suggestions-api)

## Service endpoints

### POST
* /content/suggest
Using curl:

    curl -d '{"title":"tile", "byline": "byline", "bodyXML":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp

## Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

`/__build-info`

### Logging

* The application uses [logrus](https://github.com/Sirupsen/logrus); the log file is initialised in [main.go](main.go).