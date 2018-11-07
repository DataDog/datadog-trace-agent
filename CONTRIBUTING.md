## Submitting issues

- You can first take a look at the [Troubleshooting](https://datadog.zendesk.com/hc/en-us/sections/200766955-Troubleshooting) section of our [Knowledge base](https://datadog.zendesk.com/hc/en-us).
- If you can't find anything useful, send an email to <mailto:tracehelp@datadoghq.com>, or join the [APM channel](https://datadoghq.slack.com/messages/apm) in our Datadog Slack. Visit [http://chat.datadoghq.com](http://chat.datadoghq.com) to join the Slack.
- Finally, you can open a Github issue with clear steps to reproduce. If there is an existing issue that seems close to your problem,
prefer leaving a comment there to opening a new issue.

## Pull Requests

Pull requests for bug fixes are welcome, but before submitting new features or changes to current functionalities [open an issue](https://github.com/DataDog/datadog-trace-agent/issues/new)
and discuss your ideas or propose the changes you wish to make. After a resolution is reached a PR can be submitted for review.

## Commits

For commit messages, try to use the same conventions as most Go projects, for example:
```
config: remove configurable initial pre sample rate (#489)

Fixes #113
```
Please apply the same logic for Pull Requests, start with the package name, followed by a colon and a description of the change, just like
the official [Go language](https://github.com/golang/go/pulls).
