### Background
https://github.com/Datadog/devops/wiki/Trace-Overview

#### Data Model

_From <https://cloud.google.com/trace/api/#data_model>:_

##### Span

The basic unit of work. A span describes the amount of time it takes an application to complete a suboperation in a trace. For example, it can describe how long it takes for the application to perform a round-trip RPC call to another system when handling a request, or how long it takes to perform another task that is part of a larger operation. Sending an RPC is a new span, as is sending a response to an RPC.

##### Trace

A set of spans. Traces describe the amount of time it takes an application to complete a single operation. For example, it can describe how long it takes for the application to process an incoming request from a user and return a response. Each trace consists of one or more spans, each of which describes the amount of time it takes to complete a suboperation.

##### Annotation

Annotations are used to record the existance of an event in time.

### Make it work

#### Installation (RPM)

Coming soon!

#### The Agent

1. Enable your personal-chef [godev environment](https://github.com/DataDog/devops/wiki/Development-Environment#select-your-environment)

2. Download [ES 2.0+](https://www.elastic.co/downloads/elasticsearch), extract, and run:

```
wget https://download.elastic.co/elasticsearch/release/org/elasticsearch/distribution/tar/elasticsearch/2.3.1/elasticsearch-2.3.1.tar.gz
tar xvfz elasticsearch-2.3.1.tar.gz
./elasticsearch-2.3.1/bin/elasticsearch

3. Setup ES schema
rake trace:reset_es

4. Run it
supe start trace:
```

### Instrumentation

#### Django + Flask

Coming soon!

#### The Python lib

Checkout `dogweb:dogtrace` to have access to the `dogtrace` library.

#### Snippets

```
from dogtrace.client import DogTrace

# Need to enable DogTrace to log spans
DogTrace.reporter.disabled = False

span = DogTrace.begin_span(service='flask', resource='process_request', name=['tag1','tag2'], meta={'key1':'val1'})
# ... do some work ...
DogTrace.commit_span()  
```


```
# Send a trace manually
curl "http://localhost:7777/span" -X POST -d '{"span_id": 1234, "trace_id": 46, "type": "demo", "meta": {"client":"curl", "apache.version": "2.2.2"}}'
```

