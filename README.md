### Make it work

#### The Agent

```
# Download ES 2.0+

# Setup ES schema
rake trace:reset_es

# Run it
supe start trace:
```


### The Agent UI

Simple web UI to see traces and spans. Spans need to be written in the SQLite DB.

```
pip install flask

python collector_web/server.py
```

#### The Python lib

Checkout `dogweb:dogtrace` to have access to the `dogtrace` library.

### Snippets

```
# Send a trace manually
curl "http://localhost:7777/span" -X POST -d '{"span_id": 1234, "trace_id": 46, "type": "demo", "meta": {"client":"curl", "apache.version": "2.2.2"}}'
```
