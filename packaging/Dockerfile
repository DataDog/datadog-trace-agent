FROM debian:latest

RUN apt-get update && \
    apt-get install --no-install-recommends -qq -y build-essential debhelper git gnupg rake curl devscripts rubygems ruby-dev zlib1g-dev libxml2-dev wget
RUN gem install deb-s3

RUN curl -sL -o /usr/local/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme && \
    chmod +x /usr/local/bin/gimme

ENV GOPATH=/go
ENV PATH=/go/bin:$PATH
RUN gimme 1.7.1
RUN eval $(gimme 1.7.1) && go get github.com/robfig/glock


VOLUME ["/go/src/github.com/DataDog/datadog-trace-agent"]
ENTRYPOINT ["/go/src/github.com/DataDog/datadog-trace-agent/packaging/release_deb.sh"]
