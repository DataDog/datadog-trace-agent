### Background
See https://github.com/Datadog/devops/wiki/Trace-Overview

### Instrumentation

#### Django + Flask

Coming soon!

#### The Python lib

Checkout `dogweb:dogtrace` to have access to the `dogtrace` library.

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

#### API

Coming soon!

##### Types

4 built-in types currently exist: web, DB, cache, and custom
