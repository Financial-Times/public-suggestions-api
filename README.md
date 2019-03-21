# public-suggestions-api

[![Circle CI](https://circleci.com/gh/Financial-Times/public-suggestions-api/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/public-suggestions-api/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/public-suggestions-api)](https://goreportcard.com/report/github.com/Financial-Times/public-suggestions-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/public-suggestions-api/badge.svg)](https://coveralls.io/github/Financial-Times/public-suggestions-api)

## Introduction

Provides annotation suggestions aggregated from multiple sources

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

                  --app-system-code                      System Code of the application (env $APP_SYSTEM_CODE) (default "public-suggestions-api")
                  --app-name                             Application name (env $APP_NAME) (default "public-suggestions-api")
                  --port                                 Port to listen on (env $APP_PORT) (default "8080")
                  --authors-suggestion-api-base-url      The base URL to authors suggestion api (env $AUTHORS_SUGGESTION_API_BASE_URL) (default "http://authors-suggestion-api:8080")
                  --authors-suggestion-endpoint          The endpoint for authors suggestion api (env $AUTHORS_SUGGESTION_ENDPOINT) (default "/content/suggest/authors")
                  --ontotext-suggestion-api-base-url     The base URL to ontotext suggestion api (env $ONTOTEXT_SUGGESTION_API_BASE_URL) (default "http://ontotext-suggestion-api:8080")
                  --ontotext-suggestion-endpoint         The endpoint for ontotext suggestion api (env $ONTOTEXT_SUGGESTION_ENDPOINT) (default "/content/suggest/ontotext")
                  --internal-concordances-api-base-url   The base URL for internal concordances api (env $CONCEPT_CONCORDANCES_API_BASE_URL) (default "http://internal-concordances:8080")
                  --internal-concordances-endpoint       The endpoint for internal concordances api (env $CONCEPT_CONCORDANCES_ENDPOINT) (default "/internalconcordances")
                  --public-things-api-base-url           The base URL for public things api (env $PUBLIC_THINGS_API_BASE_URL) (default "http://public-things-api:8080")
                  --public-things-endpoint               The endpoint for public things api (env $PUBLIC_THINGS_ENDPOINT) (default "/things")
                  --concept-blacklister-base-url         The base URL for concept suggester blacklister (env $CONCEPT_BLACKLISTER_BASE_URL) (default "http://concept-suggestions-blacklister:8080")
                  --concept-blacklister-endpoint         The endpoint for concept suggester blacklister (env $CONCEPT_BLACKLISTER_ENDPOINT) (default "/blacklist")          

3. Test:

    Using curl:

            curl -d '{"bodyXML":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp


## Build and deployment

* Built by Docker Hub on merge to master: [coco/public-suggestions-api](https://hub.docker.com/r/coco/public-suggestions-api/)
* CI provided by CircleCI: [public-suggestions-api](https://circleci.com/gh/Financial-Times/public-suggestions-api)

## Service endpoints

If you want to view a more comprehensive list of the endpoints available for this service check out [api.yml](_ft/api.yml).

### POST
* /content/suggest
Using curl:

    curl -d '{"title":"tile", "byline": "byline", "bodyXML":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp

### Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

`/__build-info`

`/__api`

## Logging

* The application uses [go-logger](https://github.com/Financial-Times/go-logger); the log file is initialised in [main.go](main.go).