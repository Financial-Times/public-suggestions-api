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
        govendor test -v -race
        go install

2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/draft-suggestion-api [--help]

Options:

        --app-system-code="draft-suggestion-api"            System Code of the application ($APP_SYSTEM_CODE)
        --app-name="draft-suggestion-api"                   Application name ($APP_NAME)
        --port="8080"                                           Port to listen on ($APP_PORT)
        
3. Test:

    1. Either using curl:

            curl -d '{"test":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp

    1. Or using [httpie](https://github.com/jkbrzt/httpie):

            http POST http://localhost:8080/content/suggest test=content

## Build and deployment

* Built by Docker Hub on merge to master: [coco/draft-suggestion-api](https://hub.docker.com/r/coco/draft-suggestion-api/)
* CI provided by CircleCI: [draft-suggestion-api](https://circleci.com/gh/Financial-Times/draft-suggestion-api)

## Service endpoints

e.g.
### GET

Using curl:

    curl -d '{"test":"content"}' -H "Content-Type: application/json" -X POST http://localhost:8080/content/suggest | json_pp

Or using [httpie](https://github.com/jkbrzt/httpie):

    http POST http://localhost:8080/content/suggest test=content

The expected response will contain information about the person, and the organisations they are connected to (via memberships).

Based on the following [google doc](https://docs.google.com/document/d/1SC4Uskl-VD78y0lg5H2Gq56VCmM4OFHofZM-OvpsOFo/edit#heading=h.qjo76xuvpj83).

## Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

`/__build-info`

### Logging

* The application uses [logrus](https://github.com/Sirupsen/logrus); the log file is initialised in [main.go](main.go).