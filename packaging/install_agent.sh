#!/bin/bash
# (C) Datadog, Inc. 2010-2016
# All rights reserved
# Licensed under Simplified BSD License (see LICENSE)
# Datadog Trace Agent installation script: install and set up the Trace Agent on supported Linux distributions
# using the package manager and Datadog repositories.

set -e
logfile="dd-trace-agent-install.log"

# Set up a named pipe for logging
npipe=/tmp/$$.tmp
mknod $npipe p

# Log all output to a log for error checking
tee <$npipe $logfile &
exec 1>&-
exec 1>$npipe 2>&1
trap "rm -f $npipe" EXIT


function on_error() {
    printf "\033[31m$ERROR_MESSAGE
It looks like you hit an issue when trying to install the Trace Agent.

Please send an email to trace-help@datadoghq.com
with the contents of dd-trace-agent-install.log and we'll do our very best to help you
solve your problem.\n\033[0m\n"
}
trap on_error ERR

if [ -n "$DD_HOSTNAME" ]; then
    dd_hostname=$DD_HOSTNAME
fi

if [ -n "$DD_API_KEY" ]; then
    apikey=$DD_API_KEY
fi

if [ ! $apikey ]; then
    printf "\033[31mAPI key not available in DD_API_KEY environment variable.\033[0m\n"
    exit 1;
fi

# OS/Distro Detection
# Try lsb_release, fallback with /etc/issue then uname command
KNOWN_DISTRIBUTION="(Debian|Ubuntu|RedHat|CentOS|openSUSE|Amazon|Arista)"
DISTRIBUTION=$(lsb_release -d 2>/dev/null | grep -Eo $KNOWN_DISTRIBUTION  || grep -Eo $KNOWN_DISTRIBUTION /etc/issue 2>/dev/null || grep -Eo $KNOWN_DISTRIBUTION /etc/Eos-release 2>/dev/null || uname -s)

if [ $DISTRIBUTION = "Darwin" ]; then
    printf "\033[31mThis script does not support installing on Mac.

Please use the source install script available at https://app.datadoghq.com/trace/install#.\033[0m\n"
    exit 1;

elif [ -f /etc/debian_version -o "$DISTRIBUTION" == "Debian" -o "$DISTRIBUTION" == "Ubuntu" ]; then
    OS="Debian"
elif [ -f /etc/redhat-release -o "$DISTRIBUTION" == "RedHat" -o "$DISTRIBUTION" == "CentOS" -o "$DISTRIBUTION" == "openSUSE" -o "$DISTRIBUTION" == "Amazon" ]; then
    OS="RedHat"
# Some newer distros like Amazon may not have a redhat-release file
elif [ -f /etc/system-release -o "$DISTRIBUTION" == "Amazon" ]; then
    OS="RedHat"
# Arista is based off of Fedora14/18 but do not have /etc/redhat-release
elif [ -f /etc/Eos-release -o "$DISTRIBUTION" == "Arista" ]; then
    OS="RedHat"
fi

# Root user detection
if [ $(echo "$UID") = "0" ]; then
    sudo_cmd=''
else
    sudo_cmd='sudo'
fi

# Install the necessary package sources
if [ $OS = "RedHat" ]; then
    echo -e "\033[34m\n* Installing YUM sources for Datadog Tracing\n\033[0m"

    UNAME_M=$(uname -m)
    if [ "$UNAME_M"  == "i686" -o "$UNAME_M"  == "i386" -o "$UNAME_M"  == "x86" ]; then
        ARCHI="i386"
    else
        ARCHI="x86_64"
    fi

    if [ $ARCHI = "i386" ]; then
        printf "\033[31mThis script does not support installing on i386 architectures

    Please contact us at trace-help@datadoghq.com for further assistance.\033[0m\n"
        exit 1;
    fi

    # Versions of yum on RedHat 5 and lower embed M2Crypto with SSL that doesn't support TLS1.2
    if [ -f /etc/redhat-release ]; then
        REDHAT_MAJOR_VERSION=$(grep -Eo "[0-9].[0-9]{1,2}" /etc/redhat-release | head -c 1)
    fi
    if [ -n "$REDHAT_MAJOR_VERSION" ] && [ "$REDHAT_MAJOR_VERSION" -le "5" ]; then
        PROTOCOL="http"
    else
        PROTOCOL="https"
    fi

    $sudo_cmd sh -c "echo -e '[datadog-trace]\nname = Datadog, Inc.\nbaseurl = http://yum-trace.datad0g.com.s3.amazonaws.com/x86_64/\nenabled=1\ngpgcheck=1\npriority=1\ngpgkey=$PROTOCOL://yum.datadoghq.com/DATADOG_RPM_KEY_E09422B3.public' > /etc/yum.repos.d/datadog-trace.repo"

    printf "\033[34m* Installing the Datadog Trace Agent package\n\033[0m\n"

    $sudo_cmd yum -y --disablerepo='*' --enablerepo='datadog-trace' install dd-trace-agent || $sudo_cmd yum -y install dd-trace-agent
elif [ $OS = "Debian" ]; then
    printf "\033[34m\n* Installing APT package sources for Datadog Tracing\n\033[0m\n"
    $sudo_cmd sh -c "echo 'deb http://apt-trace.datad0g.com.s3.amazonaws.com/ stable main' > /etc/apt/sources.list.d/datadog-trace.list"
    $sudo_cmd apt-key adv --recv-keys --keyserver hkp://keyserver.ubuntu.com:80 382E94DE

    printf "\033[34m\n* Installing the Datadog Trace Agent package\n\033[0m\n"
    ERROR_MESSAGE="ERROR
Failed to update the sources after adding the Datadog Trace repository.
This may be due to any of the configured APT sources failing -
see the logs above to determine the cause.
If the failing repository is Datadog, please contact Datadog support.
*****
"
    $sudo_cmd apt-get update -o Dir::Etc::sourcelist="sources.list.d/datadog-trace.list" -o Dir::Etc::sourceparts="-" -o APT::Get::List-Cleanup="0"
    ERROR_MESSAGE="ERROR
Failed to install the Datadog Trace Agent package, sometimes it may be
due to another APT source failing. See the logs above to
determine the cause.
If the cause is unclear, please contact trace-help@datadoghq.com.
*****
"
    $sudo_cmd apt-get install -y --force-yes dd-trace-agent
    ERROR_MESSAGE=""
else
    printf "\033[31mYour OS or distribution is not supported by this install script.
Please follow the instructions on the Agent setup page:

    https://app.datadoghq.com/trace/install\033[0m\n"
    exit 1;
fi

# Check the configuration file we need exists
if [ -e /etc/dd-agent/datadog.conf ]; then
    printf "\033[34m\n* Detected datadog.conf configuration file\n\033[0m\n"
else
    printf "\033[31m
We could not locate a valid configuration file for dd-agent.

The Trace Agent may not function correctly without dd-agent running on your machine

Please contact us at trace-help@datadoghq.com for further assistance.\033[0m\n"
fi

# echo some instructions and exit
printf "\033[32m

Your Agent is running and functioning properly. It will continue to run in the
background and submit traces to Datadog from your local applications.

Logs for the agent can be found under /var/log/datadog/trace-agent.log

If you ever want to stop the Agent, run:

    sudo service dd-trace-agent stop

And to run it again run:

    sudo service dd-trace-agent start

\033[0m"
