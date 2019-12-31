# Logspout Image For New Relic
[![Build Status](https://circleci.com/gh/aminoz007/logspout.svg?style=svg)](https://circleci.com/gh/aminoz007/logspout)
[![Go Report Card](https://goreportcard.com/badge/github.com/aminoz007/logspout?style=flat-square)](https://goreportcard.com/report/github.com/aminoz007/logspout)
[![GoDoc](https://godoc.org/github.com/aminoz007/logspout?status.svg)](https://godoc.org/github.com/aminoz007/logspout)
[![Release](https://img.shields.io/github/release/aminoz007/logspout.svg?style=flat-square)](https://github.com/aminoz007/logspout/releases/latest)

This is a [Logspout](https://github.com/gliderlabs/logspout) custom image that forwards all your containers logs to New Relic via HTTP POST using New Relic Logs API.

This project is provided AS-IS WITHOUT WARRANTY OR SUPPORT, although you can report issues and contribute to the project here on GitHub.

## Usage

### Docker

```bash 
docker run --name="newrelic" --restart=always \
-d -v=/var/run/docker.sock:/var/run/  docker.sock \
-e "<KEY>=<KEY_VALUE>" aminoz86/logspout-newrelic:latest
```
Where `<KEY>` is exactly one of the following:

| Property | Description |
|---|---|
| api_key | Your New Relic API Insert Key |
| license_key | Your New Relic License Key |

### Elastic Container Service (ECS)

Update your ECS Cloud Services Configuration as detailed below:
```yaml
services:
  newrelic:
    environment:
        - <KEY>="<KEY_VALUE>"
    image: aminoz86/logspout-newrelic:latest
    restart: always
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    deploy:
      mode: global
```

### Docker Swarm

Update your Docker Swarm Compose file as detailed below:
```yaml
version: "3"
networks:
  logging:
services:
  newrelic:
    image: aminoz86/logspout-newrelic:latest
    networks:
      - logging
    volumes:
      - /etc/hostname:/etc/host_hostname:ro
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - <KEY>="<KEY_VALUE>"
    deploy:
      mode: global
```

## Configuration

###  Image configuration: Environment Variables


| Property | Description | Default Value | Required or Optional
|---|---|---|---|
| API_KEY | Your New Relic API Insert key | | Required if `LICENSE_KEY` is not provided
| LICENSE_KEY | Your New Relic License key | | Required if `API_KEY` is not provided
| NEW_RELIC_URL | New Relic ingestion endpoint | `https://log-api.newrelic.com/log/v1` | Optional
| PROXY_URL | Use proxy endpoint to send the data to NR | | Optional
| FILTER_NAME | Filter by container name with wildcards. For more information, review logspout docs [here!](https://github.com/gliderlabs/logspout#including-specific-containers) | | Optional
| FILTER_ID | Filter by container ID with cildcards. For more information, review logspout docs [here!](https://github.com/gliderlabs/logspout#including-specific-containers) | | Optional
| FILTER_SOURCES | Filter by comma-separated list of sources. For more information, review logspout docs [here!](https://github.com/gliderlabs/logspout#including-specific-containers) | | Optional
| FILTER_LABELS | Filter by comma-separated list of labels. For more information, review logspout docs [here!](https://github.com/gliderlabs/logspout#including-specific-containers) | | Optional
| HOSTNAME | Use this variable to overwrite default `Hostname` | {{Container.Config.Hostname}} |Optional|
| VERBOSE | Capture or not logspout container's logs | Enabled (set to `0` to disable) | Optional
| FLUSH_INTERVAL | Harvest cycle (in **milliseconds**) | 250 | Optional
| MAX_BUFFER_SIZE | The maximum size of logs for each POST request (in **mb**) | 1 | Optional
| MAX_LINE_LENGTH | The maximum length for each log message (it gets truncated if it is bigger than this limit) | 15000 | Optional
| MAX_REQUEST_RETRY | The maximum number of retries for sending a batch of logs when there are network failures | 5 | Optional
| INACTIVITY_TIMEOUT | Logspout relies on the Docker API to retrieve container logs. A failure in the API may cause a log stream to hang. Logspout can detect and restart inactive Docker log streams. Use the environment variable `INACTIVITY_TIMEOUT` to enable this feature. E.g.: `INACTIVITY_TIMEOUT=1m` for a 1-minute threshold. | 1m | Optional


### EU image configuration

If you are running this image in the EU set the `NEW_RELIC_URL` to `https://log-api.eu.newrelic.com/log/v1`.

### Getting Your Keys

* Getting your New Relic Insights Insert key:
`https://insights.newrelic.com/accounts/<ACCOUNT_ID>/manage/api_keys`

* Getting your New Relic license key:
`https://rpm.newrelic.com/accounts/<ACCOUNT_ID>`

## Issues / Enhancement Requests

Issues and enhancement requests can be submitted in the [Issues tab of this repository](https://github.com/aminoz007/logspout/issues).
Please search for and review the existing open issues before submitting a new issue.

## Contributing

Contributions are welcome (and if you submit a Enhancement Request, expect to be invited to
contribute it yourself :grin:).

