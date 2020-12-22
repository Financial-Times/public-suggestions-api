# UPP - Public Suggestions API

A service serving requests made towards the suggestions umbrella.

## Code

public-suggestions-api

## Primary URL

api.ft.com/content/suggest

## Service Tier

Bronze

## Lifecycle Stage

Production

## Delivered By

content

## Supported By

content

## Known About By

- elitsa.pavlova
- ivan.nikolov
- marina.chompalova
- miroslav.gatsanoga
- kalin.arsov

## Host Platform

AWS

## Architecture

Provides annotation suggestions aggregated from multiple sources.

## Contains Personal Data

No

## Contains Sensitive Data

No

## Dependencies

- author-suggestion-api
- concept-suggestions-blacklister
- ontotext-suggestion-api
- internal-concordances
- public-things-api

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

PartiallyAutomated

## Failover Details

The failover guide for the cluster is located [here](https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/delivery-cluster).

## Data Recovery Process Type

NotApplicable

## Data Recovery Details

The service does not store data, so it does not require any data recovery steps.

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

The release is triggered by making a Github release which is then picked up by a Jenkins multibranch pipeline. The Jenkins pipeline should be manually started in order to deploy the helm package to the Kubernetes clusters.

## Key Management Process Type

NotApplicable

## Key Management Details

There is no key rotation procedure for this system.

## Monitoring

Look for the pods in the cluster health endpoint and click to see pod health and checks:

- <https://upp-prod-delivery-eu.upp.ft.com/__health>
- <https://upp-prod-delivery-us.upp.ft.com/__health>

## First Line Troubleshooting

<https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting>

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
