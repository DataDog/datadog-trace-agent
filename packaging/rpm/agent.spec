%define _builddir /go/src/github.com/DataDog/datadog-trace-agent
%define _rpmdir /go/src/github.com/DataDog/datadog-trace-agent/RPMS

Name: dd-trace-agent
Version: 0.99.0
Requires: datadog-agent >= 1:5.8.0-1
Release: 1
License: BSD
Summary: A tracing agent crafted with <3 from Datadog
Buildroot: /go/src/github.com/DataDog/datadog-trace-agent
Packager: Datadog <dev@datadoghq.com>
BuildRequires: systemd

%description
Datadog's tracing agent

%prep
rake restore

%build
TRACE_AGENT_VERSION=$RPM_PACKAGE_VERSION rake build

%install
mkdir -p $RPM_BUILD_ROOT/opt/datadog-agent/bin
mkdir -p $RPM_BUILD_ROOT/etc/init.d

install -m 755 trace-agent $RPM_BUILD_ROOT/opt/datadog-agent/bin/trace-agent
install -m 755 packaging/rpm/dd-trace-agent.init $RPM_BUILD_ROOT/etc/init.d/dd-trace-agent


%post
# start the service
$RPM_BUILD_ROOT/etc/init.d/dd-trace-agent start

%systemd_post packaging/rpm/dd-trace-agent.service

%preun
%systemd_preun packaging/rpm/dd-trace-agent.service

%postun
%systemd_postun_with_restart packaging/rpm/dd-trace-agent.service

%files
/opt/datadog-agent/bin/trace-agent
/etc/init.d/dd-trace-agent
