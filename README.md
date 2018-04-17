# StackState APM agent

[![CircleCI](https://circleci.com/gh/StackVista/stackstate-trace-agent.svg?style=svg)](https://circleci.com/gh/StackVista/stackstate-trace-agent)

An agent that collects traces from various sources, normalizes and pre-processes them before sending the info to the StackState backend.


## Run on Linux

The Trace Agent is packaged with the standard StackState Agent.
Just [run the StackState Agent](http://docs.stackstatehq.com/guides/basic_agent_usage/).

Note: the Trace Agent is not yet included in the installation from source of
the Trace Agent. Follow the instructions in [Development](#development) to do
it manually.


## Run on OSX

The APM agent (aka Trace Agent) isn't part of the OSX StackState Agent yet, it needs to be run manually on the side.

- Have the [OSX StackState Agent](https://app.stackstatehq.com/account/settings#agent/mac).
- Download the [latest OSX Trace Agent release](https://github.com/StackVista/stackstate-trace-agent/releases/latest).
- Run the Trace Agent using the StackState Agent configuration.

    `./trace-agent-osx-X.Y.Z -config /opt/stackstate-agent/etc/stackstate.conf`

- The Trace Agent should now be running in foreground, with an initial output similar to:

```
2017-04-24 13:46:35 INFO (main.go:166) - using configuration from /opt/stackstate-agent/etc/stackstate.conf
2017-04-24 13:46:36 INFO (agent.go:200) - Failed to parse hostname from sts-agent config
2017-04-24 13:46:36 DEBUG (agent.go:288) - No aggregator configuration, using defaults
2017-04-24 13:46:36 INFO (main.go:220) - trace-agent running on host My-MacBook-Pro.local
2017-04-24 13:46:36 INFO (receiver.go:137) - listening for traces at http://localhost:8126
```

## Run on Windows

On Windows, the trace agent is shipped together with the StackState Agent only
since version 5.19.0, so users must update to 5.19.0 or above. However the
Windows trace agent is in beta and some manual steps are required.

Update your config file to include:

```
[Main]
apm_enabled: yes
[trace.config]
log_file = C:\ProgramData\StackState\logs\trace-agent.log
```

Restart the stackstateagent service:

```
net stop stackstateagent
net start stackstateagent
```

For this beta the trace agent status and logs are not displayed in the Agent
Manager GUI.

To see the trace agent status either use the Service tab of the Task Manager or
run:

```
sc.exe query stackstate-trace-agent
```

And check that the status is "running".

Note: the Trace Agent is not yet included in the installation from source of
the Trace Agent. Follow the instructions in [Development](#development) to do
it manually.

## Development

Pre-requisites:
- `go` 1.9+


Build and run from source:
- Run `make install` to install the `trace-agent` binary in $GOPATH/bin
- You may then run it with `trace-agent --config PATH_TO_YOUR_DATADOG_CONFIG_FILE`


## Testing

- We use [`golint`](https://github.com/golang/lint) to lint our source code.
- You may also run the CI locally using the [CircleCI CLI](https://circleci.com/docs/2.0/local-jobs/): `circleci build`.
- To run only the tests, simply run `go test ./...`


## Contributing

See our [contributing guidelines](CONTRIBUTING.md)

More detailed information about agent configuration, terminology and architecture can be found in our [wiki](https://github.com/StackVista/stackstate-trace-agent/wiki)
