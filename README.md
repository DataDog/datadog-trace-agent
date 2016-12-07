# Datadog APM agent

An agent that collects traces from various sources, normalizes and pre-processes them before sending the info to the Datadog backend.

## Development

Pre-requisites:
- `go` 1.7+
- `rake`


Hacking:
- Sync your Go dependencies with `rake restore`. This uses [glock](https://github.com/robfig/glock) to sync all deps listed in the [GLOCKFILE](GLOCKFILE)
- Run `rake build` to build the `trace-agent` binary from current source
- Or run `rake install` to install `trace-agent` to your $GOPATH

## Testing
- Lint with `rake lint`
- Run the full CI suite locally with `rake ci`
- Alternatively test individual packages like so `go test ./agent`

## Contributing

See our [contributing guidelines](CONTRIBUTING.md)

More detailed information about agent configuration, terminology and architecture can be found in our [wiki](https://github.com/DataDog/datadog-trace-agent/wiki)



