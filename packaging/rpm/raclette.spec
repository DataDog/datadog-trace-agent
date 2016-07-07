%define _builddir /go/src/github.com/DataDog/raclette
%define _rpmdir /go/src/github.com/DataDog/raclette/RPMS

Name: dd-trace-agent
Version: 0.99.0
Requires: datadog-agent >= 1:5.8.0-1
Release: 1
License: BSD
Summary: A tracing agent crafted with <3 from Datadog
Buildroot: /go/src/github.com/DataDog/raclette
Packager: Datadog <dev@datadoghq.com>

%description
Datadog's tracing agent

%prep
rm -rf ./RPMS/
rake restore
mkdir -p ./RPMS

%build
TRACE_AGENT_VERSION=$RPM_PACKAGE_VERSION rake build

%install
mkdir -p $RPM_BUILD_ROOT/opt/datadog-agent/bin
mv trace-agent $RPM_BUILD_ROOT/opt/datadog-agent/bin/trace-agent

%files
/opt/datadog-agent/bin/trace-agent
