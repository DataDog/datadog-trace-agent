# Datadog APM agent

[![CircleCI](https://circleci.com/gh/DataDog/datadog-trace-agent.svg?style=svg)](https://circleci.com/gh/DataDog/datadog-trace-agent)

An agent that collects traces from various sources, normalizes and pre-processes them before sending the info to the Datadog backend.


## Run on Linux

The Trace Agent is packaged with the standard Datadog Agent.
Just [run the Datadog Agent](http://docs.datadoghq.com/guides/basic_agent_usage/).

Note: the Trace Agent is not yet included in the installation from source of
the Trace Agent. Follow the instructions in [Development](#development) to do
it manually.


## Run on macOS

The APM agent (aka Trace Agent) isn't part of the OSX Datadog Agent yet, it needs to be run manually on the side.

- Have the [OSX Datadog Agent](https://app.datadoghq.com/account/settings#agent/mac).
- Download the [latest OSX Trace Agent release](https://github.com/DataDog/datadog-trace-agent/releases/latest).
- Run the Trace Agent using the Datadog Agent configuration.

    `./trace-agent-darwin-amd64-X.Y.Z -config /opt/datadog-agent/etc/datadog.yaml`

- The Trace Agent should now be running in foreground, with an initial output similar to:

```
2017-04-24 13:46:35 INFO (main.go:166) - Loaded configuration: /opt/datadog-agent/etc/datadog.yaml
2017-04-24 13:46:36 INFO (agent.go:200) - Failed to parse hostname from dd-agent config
2017-04-24 13:46:36 DEBUG (agent.go:288) - No aggregator configuration, using defaults
2017-04-24 13:46:36 INFO (main.go:220) - trace-agent running on host My-MacBook-Pro.local
2017-04-24 13:46:36 INFO (receiver.go:137) - listening for traces at http://localhost:8126
```

## Run on Windows

On Windows, the trace agent is shipped together with the Datadog Agent only
since version 5.19.0, so users must update to 5.19.0 or above. However the
Windows trace agent is in beta and some manual steps are required.

Update your config file to include:

```
[Main]
apm_enabled: yes
[trace.config]
log_file = C:\ProgramData\Datadog\logs\trace-agent.log
```

Restart the datadogagent service:

```
net stop datadogagent
net start datadogagent
```

For this beta the trace agent status and logs are not displayed in the Agent
Manager GUI.

To see the trace agent status either use the Service tab of the Task Manager or
run:

```
sc.exe query datadog-trace-agent
```

And check that the status is "running".

Note: the Trace Agent is not yet included in the installation from source of
the Trace Agent. Follow the instructions in [Development](#development) to do
it manually.

## Development

First, make sure Go 1.9+ is installed. You can do this by following the steps on the [official website](https://golang.org/dl/).

Then, download (or update) the latest source by running:
```bash
go get -u github.com/DataDog/datadog-trace-agent/...
```
Navigate to the download location and install the `trace-agent` binary in $GOPATH/bin:
```bash
cd $GOPATH/src/github.com/DataDog/datadog-trace-agent
make install
```
You may now run the agent using `$GOPATH/bin/trace-agent`.


## Testing

- We use [`golint`](https://github.com/golang/lint) to lint our source code.
- You may also run the CI locally using the [CircleCI CLI](https://circleci.com/docs/2.0/local-jobs/): `circleci build`.
- To run only the tests, simply run `go test ./...`


## Contributing

See our [contributing guidelines](CONTRIBUTING.md)

More detailed information about agent configuration, terminology and architecture can be found in our [wiki](https://github.com/DataDog/datadog-trace-agent/wiki)
