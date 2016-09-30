FROM debian:latest

RUN apt-get update && \
    apt-get install -y rake && \
    apt-get install -y debhelper && \
    apt-get install -y git

RUN curl -sL -o /gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme && \
    chmod +x /gimme

RUN echo "#!/bin/sh \n $(/gimme 1.7.1) && cd /go/src/github.com/DataDog/raclette/packaging && dpkg-buildpackage -us -uc -b" >> /build.sh
RUN chmod +x /build.sh

VOLUME ["/go/src/github.com/DataDog/raclette"]
ENV GOPATH=/go
ENV PATH=/go/bin:$PATH
ENTRYPOINT ["/build.sh"]
