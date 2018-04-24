# StackState APM agent

[![CircleCI](https://circleci.com/gh/StackVista/stackstate-trace-agent.svg?style=svg)](https://circleci.com/gh/StackVista/stackstate-trace-agent)

An agent that collects traces from various sources, normalizes and pre-processes them before sending the info to the StackState backend.


## Run on Linux

The Trace Agent is packaged with the standard StackState Agent.
Just [run the StackState Agent](http://docs.stackstate.com/guides/basic_agent_usage/).

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
