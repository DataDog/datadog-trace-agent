# Agent Configuration

The trace-agent sources configuration from the following locations:

1. The StackState Agent configuration file, provided to the `-config` command line flag (default: `/etc/stackstate/stackstate.conf`)
2. Environment variables: See full list below

Environment variables will override settings defined in configuration files.

## File configuration

Refer to the [StackState Agent example configuration](https://github.com/StackVista/dd-agent/blob/master/stackstate.conf.example) to see all available options.


## Environment variables
We allow overriding a subset of configuration values from the environment. These
can be useful when running the agent in a Docker container or in other situations
where env vars are preferrable to static files

- `STS_APM_ENABLED` - overrides `[Main] apm_enabled`
- `STS_HOSTNAME` - overrides `[Main] hostname`
- `STS_API_KEY` - overrides `[Main] api_key`
- `STS_DOGSTATSD_PORT` - overrides `[Main] dogstatsd_port`
- `STS_BIND_HOST` - overrides `[Main] bind_host`
- `STS_APM_NON_LOCAL_TRAFFIC` - overrides `[Main] non_local_traffic`
- `STS_LOG_LEVEL` - overrides `[Main] log_level`
- `STS_RECEIVER_PORT` - overrides `[trace.receiver] receiver_port`
- `STS_IGNORE_RESOURCE` - overrides `[trace.ignore] resource`
