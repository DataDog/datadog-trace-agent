# Datadog APM agent

[![CircleCI](https://circleci.com/gh/DataDog/datadog-trace-agent.svg?style=svg)](https://circleci.com/gh/DataDog/datadog-trace-agent)

An agent that collects traces from various sources, normalizes and pre-processes them before sending the info to the Datadog backend.


## Run on Linux

The Trace Agent is packaged with the standard Datadog Agent.
Just [run the Datadog Agent](http://docs.datadoghq.com/guides/basic_agent_usage/).

Note: the Trace Agent is not yet included in the installation from source of
the Trace Agent. Follow the instructions in [Development](#development) to do
it manually.


## Run on OSX

The APM agent (aka Trace Agent) isn't part of the OSX Datadog Agent yet, it needs to be run manually on the side.

- Have the [OSX Datadog Agent](https://app.datadoghq.com/account/settings#agent/mac).
- Download the [latest OSX Trace Agent release](https://github.com/DataDog/datadog-trace-agent/releases/latest).
- Run the Trace Agent using the Datadog Agent configuration.

    `./trace-agent-osx-X.Y.Z -config /opt/datadog-agent/etc/datadog.conf`

- The Trace Agent should now be running in foreground, with an initial output similar to:

```
2017-04-24 13:46:35 INFO (main.go:166) - using configuration from /opt/datadog-agent/etc/datadog.conf
2017-04-24 13:46:36 INFO (agent.go:200) - Failed to parse hostname from dd-agent config
2017-04-24 13:46:36 DEBUG (agent.go:288) - No aggregator configuration, using defaults
2017-04-24 13:46:36 INFO (main.go:220) - trace-agent running on host My-MacBook-Pro.local
2017-04-24 13:46:36 INFO (receiver.go:137) - listening for traces at http://localhost:8126
```

## Run on Windows

- Have the [Windows Datadog Agent](https://app.datadoghq.com/account/settings#agent/windows).
- Download the [latest Windows Trace Agent release](https://github.com/DataDog/datadog-trace-agent/releases/latest).
- Run the Trace Agent using the Datadog Agent configuration.

    `./trace-agent-windows-X.Y.Z.exe -config "%ProgramData%\Datadog\datadog.conf"`


## Development

Pre-requisites:
- `go` 1.9+
- [glide](https://github.com/Masterminds/glide)


Build and run from source:
- Install dependencies by running `glide install`
- Run `go install ./cmd/...` to install the `trace-agent` binary in $GOPATH/bin
- You may then run it with `trace-agent --config PATH_TO_YOUR_DATADOG_CONFIG_FILE`


## Testing

- We use [`golint`](https://github.com/golang/lint) to lint our source code.
- You may also run the CI locally using the [CircleCI CLI](https://circleci.com/docs/2.0/local-jobs/): `circleci build`.
- To run only the tests, simply run `go test ./...`


## Contributing

See our [contributing guidelines](CONTRIBUTING.md)

More detailed information about agent configuration, terminology and architecture can be found in our [wiki](https://github.com/DataDog/datadog-trace-agent/wiki)
