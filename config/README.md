# Agent Configuration

The trace-agent sources configuration from the following locations:

1. The path pointed to by the `-ddconfig` command line flag (default: `/etc/dd-agent/datadog.conf`)
2. The path pointed to by the `-config` command line flag (default: `/etc/datadog/trace-agent.ini`)
3. Environment variables: See full list below


Environment variables will override settings defined in configuration files.

## Classic configuration values, and how the trace-agent treats them
Note that changing these will also change the behavior of the `datadog-agent` running on the same host.

In the file pointed to by `-ddconfig`

```
[Main]
# Enable the trace agent.
apm_enabled = true

# trace-agent will use this hostname when reporting to the Datadog backend.
# default: stdout of `hostname`
hostname = myhost

# trace-agent will use this api key when reporting to the Datadog backend.
# no default.
api_key =

# trace-agent will bind to this host when listening for traces
# additionally trace-agent expects dogstatsd to be bound to the same host
# for forwarding internal monitoring metrics
bind_host = 127.0.0.1

# trace-agent expects dogstatsd to be listening over UDP on this port
# this is where it will forward internal monitoring metrics
dogstatsd_port = 8125

# trace-agent will log it's output with this log level
log_level = INFO
```

## APM-specific configuration values
In the file pointed to by `-config`

```
[trace.sampler]
# Extra global sample rate to apply on all the traces
# This sample rate is combined to the sample rate from the sampler logic, still promoting interesting traces
# From 1 (no extra rate) to 0 (don't sample at all)
extra_sample_rate=1

# Maximum number of traces per second to sample.
# The limit is applied over an average over a few minutes ; much bigger spikes are possible.
# Set to 0 to disable the limit.
max_traces_per_second=10

[trace.receiver]
# the port that the Receiver should listen on
receiver_port=8126
# how many unique client connections to allow during one 30 second lease period
connection_limit=2000

```


## Environment variables
We allow overriding a subset of configuration values from the environment. These
can be useful when running the agent in a Docker container or in other situations
where env vars are preferrable to static files

- `DD_APM_ENABLED` - overrides `[Main] apm_enabled`
- `DD_HOSTNAME` - overrides `[Main] hostname`
- `DD_API_KEY` - overrides `[Main] api_key`
- `DD_DOGSTATSD_PORT` - overrides `[Main] dogstatsd_port`
- `DD_BIND_HOST` - overrides `[Main] bind_host`
- `DD_LOG_LEVEL` - overrides `[Main] log_level`
- `DD_RECEIVER_PORT` - overrides `[trace.receiver] receiver_port`


## Logging
Unlike dd-agent, the trace-agent does not configure it's own logging and relies on the process manager
to redirect it's output. While standard installs (`apt-get`, `yum`) will log output to `/var/log/datadog/trace-agent.log`,
any non-standard install should attempt to handle STDERR in a sane way
