#~ /bin/bash

# Soup-to-nuts build script for the agent deb
# Run on a Docker base ubuntu image with the source tree mounted at /go/src/github.com/DataDog/raclette

apt-get update
apt-get install -y rake
apt-get install -y debhelper
apt-get install -y git

raclettepath=/go/src/github.com/DataDog/raclette/packaging

url=https://storage.googleapis.com/golang/go1.4.2.linux-amd64.tar.gz
out=go.tar.gz
curl -XGET $url -o $out
mkdir -p /usr/local/
tar zxfv $out -C /usr/local/

export GOPATH=/go
export PATH=$PATH:/usr/local/go/bin/
export PATH=$PATH:$GOPATH/bin/

cd /go/src/github.com/DataDog/raclette/packaging
dpkg-buildpackage -us -uc -b
