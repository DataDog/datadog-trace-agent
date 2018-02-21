# This Makefile is used within the release process of the main Datadog Agent to pre-package datadog-trace-agent:
# https://github.com/DataDog/datadog-agent/blob/2b7055c/omnibus/config/software/datadog-trace-agent.rb

# if the TRACE_AGENT_VERSION environment variable isn't set, default to 0.99.0
TRACE_AGENT_VERSION := $(if $(TRACE_AGENT_VERSION),$(TRACE_AGENT_VERSION), 0.99.0)

# break up the version
SPLAT = $(subst ., ,$(TRACE_AGENT_VERSION))
VERSION_MAJOR = $(word 1, $(SPLAT))
VERSION_MINOR = $(word 2, $(SPLAT))
VERSION_PATCH = $(word 3, $(SPLAT))

# account for some defaults
VERSION_MAJOR := $(if $(VERSION_MAJOR),$(VERSION_MAJOR), 0)
VERSION_MINOR := $(if $(VERSION_MINOR),$(VERSION_MINOR), 0)
VERSION_PATCH := $(if $(VERSION_PATCH),$(VERSION_PATCH), 0)

deps: clean-deps
	# downloads and installs dependencies
	go get -d github.com/Masterminds/glide/...
	# use a known version
	cd $(GOPATH)/src/github.com/Masterminds/glide && git reset --hard v0.12.3 && cd -
	# install it
	go install github.com/Masterminds/glide/...
	# get all dependencies
	glide install

install: deps clean-install
	# prepares all dependencies by running the 'deps' task, generating
	# versioning information and installing the binary.
	go generate ./info
	go install ./cmd/trace-agent

ci: deps
	# task used by CI
	go get -u github.com/golang/lint/golint/...
	golint ./cmd/trace-agent ./filters ./fixtures ./info ./quantile ./quantizer ./sampler ./statsd ./watchdog ./writer
	go test ./...

windows: clean-windows
	# pre-packages resources needed for the windows release
	windmc --target pe-x86-64 -r cmd/trace-agent/windows_resources cmd/trace-agent/windows_resources/trace-agent-msg.mc
	windres --define MAJ_VER=$(VERSION_MAJOR) --define MIN_VER=$(VERSION_MINOR) --define PATCH_VER=$(VERSION_PATCH) -i cmd/trace-agent/windows_resources/trace-agent.rc --target=pe-x86-64 -O coff -o cmd/trace-agent/rsrc.syso

clean: clean-deps clean-install clean-windows

clean-deps:
	rm -rf vendor

clean-windows:
	rm -f ./cmd/trace-agent/windows_resources/*.bin ./cmd/trace-agent/windows_resources/trace-agent-msg.rc

clean-install:
	rm -f ./info/git_version.go
