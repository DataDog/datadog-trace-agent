import json
import time
import random

from .client import Client


class Span(object):

    def __init__(
        self,
        trace_id=None, span_id=None, parent_id=None,
        start=None, duration=None, sample_size=None,
        span_type=None, service=None, resource=None, meta=None,
    ):
        self.trace_id = trace_id
        self.span_id = span_id
        self.parent_id = parent_id
        self.start = start
        self.duration = duration
        self.sample_size = sample_size
        self.type = span_type
        self.service = service
        self.resource = resource
        self.meta = meta

        if self.trace_id is None:
            self.trace_id = new_trace_id()
        if self.span_id is None:
            self.span_id = new_span_id()
        if self.start is None:
            self.start = time.time()
        if self.sample_size is None:
            self.sample_size = 1
        if self.meta is None:
            self.meta = {}

        # Automatically register the span into the client
        Client.add_span(self)

    def add_meta(self, name, value):
        self.meta[name] = unicode(value)

    def serialize(self):
        json_span = {}
        if self.trace_id:
            json_span['trace_id'] = self.trace_id
        if self.span_id:
            json_span['span_id'] = self.span_id
        if self.parent_id:
            json_span['parent_id'] = self.parent_id
        if self.start:
            json_span['start'] = self.start
        if self.duration:
            json_span['duration'] = self.duration
        if self.sample_size:
            json_span['sample_size'] = self.sample_size
        if self.type:
            json_span['type'] = self.type
        if self.service:
            json_span['service'] = self.service
        if self.resource:
            json_span['resource'] = self.resource
        if self.meta:
            json_span['meta'] = self.meta

        return json.dumps(json_span)

    def close(self):
        # Close the span when not done manually
        if not self.duration:
            self.duration = time.time() - self.start

    def create_child(self, **kwargs):
        return Span(trace_id=self.trace_id, parent_id=self.span_id, start=time.time(), **kwargs)


def new_span(span_type=None):
    return Span(trace_id=new_trace_id(), span_id=new_span_id(), start=time.time(), span_type=span_type)


def new_trace_id():
    return random.getrandbits(63)


def new_span_id():
    return random.getrandbits(31)
