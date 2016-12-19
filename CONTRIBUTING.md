# Contributing to Datadog Trace Agent

:tada: First of all, thanks for contributing! :tada:

This document aims to provide some basic guidelines to contribute to this repository, but keep in mind that these are just guidelines, not rules; use your best judgment and feel free to propose changes to this document in a pull request.

## Submitting issues

- You can first take a look at the [Troubleshooting](https://datadog.zendesk.com/hc/en-us/sections/200766955-Troubleshooting) section of our [Knowledge base](https://datadog.zendesk.com/hc/en-us).
- If you can't find anything useful, send an email to <mailto:tracehelp@datadoghq.com>, or contact us via Slack at https://dd.slack.com/messages/trace-help
- Finally, you can open a Github issue with clear steps to reproduce. If there is an existing issue that seems close to your problem,
prefer leaving a comment there to opening a new issue.


## Pull Requests

In order to ease/speed up our review, here are some items you can check/improve when submitting your PR:

- [ ] have a [proper commit history](#commits) (we advise you to rebase if needed).
- [ ] write tests for the code you wrote.
- [ ] preferably make sure that all tests pass locally
- [ ] summarize your PR with a good title and a message describing your changes, cross-referencing any related bugs/PRs.

Your Pull Request **must** always pass the CircleCI tests before being merged. If you think the error is not due to your changes, you can have a talk with us at https://dd.slack.com/messages/trace-help

## Commits

If your commit is only shipping documentation changes or example files, and is a complete no-op for the test suite, please add **[skip ci]** in the commit message body to skip the build and give your build slot to someone else _in need :wink:_

Please respect the existing commit title convention:
```
category: short description of matter
```

E.g.

- config: respect DD_LOG_LEVEL env var
- agent: explicit message when no API key is given
- quantizer: add test for multiline queries
