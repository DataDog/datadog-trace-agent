#!/bin/bash -e

# Expects gimme to be installed
eval "$(gimme 1.7.1)"

export DEBFULLNAME="Datadog, Inc"
export DEBEMAIL="package@datadoghq.com"

# Soup-to-nuts build script for the agent deb
# Run on a Docker base ubuntu image with the source tree mounted at /go/src/github.com/DataDog/datadog-trace-agent
agentpath=/go/src/github.com/DataDog/datadog-trace-agent/packaging
cd $agentpath


# increment the version number to what we want
dch --empty -v $TRACE_AGENT_VERSION --distribution $DISTRO "New beta agent release"
dpkg-buildpackage -b -us -uc

echo -e $SIGNING_PRIVATE_KEY | gpg --import

debsign -p "gpg --passphrase ${INPUT_GPG_PASSPHRASE} --batch --no-use-agent" ../dd-trace-agent_${TRACE_AGENT_VERSION}_amd64.changes
gpg --verify ../dd-trace-agent_${TRACE_AGENT_VERSION}_amd64.changes
if [ $? -ne 0 ]; then
    echo "Signature is bad, exiting"
    exit 1
fi


# make sure we're not uploading a dupe
set +e
wget --spider ${APT_BASE_PATH}/dd-trace-agent_${TRACE_AGENT_VERSION}_amd64.deb
if [ $? -eq 0 ]; then
    echo "Duplicate version detected, exiting"
    exit 1
fi
set -e

DEBFILE=`find ../ -type f -name '*.deb'`
echo $INPUT_GPG_PASSPHRASE | deb-s3 upload --bucket apt-trace.datad0g.com -c $DISTRO -m main --arch amd64 --sign=$SIGN_KEY_ID --gpg_options="--passphrase-fd 0 --no-tty" $DEBFILE --preserve-versions
