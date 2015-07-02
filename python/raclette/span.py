import json
import time
import random

from .client import Client


class Span(object):

    def __init__(
        self,
        trace_id=None, span_id=None, parent_id=None,
        start=None, end=None, duration=None,
        span_type=None, meta=None,
    ):
        self.trace_id = trace_id
        self.span_id = span_id
        self.parent_id = parent_id
        self.start = start
        self.end = end
        self.duration = duration
        self.type = span_type
        self.meta = meta

        if self.trace_id is None:
            self.trace_id = new_trace_id()
        if self.span_id is None:
            self.span_id = new_span_id()
        if self.start is None:
            self.start = time.time()
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
        if self.end:
            json_span['end'] = self.end
        if self.duration:
            json_span['duration'] = self.duration
        if self.type:
            json_span['type'] = self.type
        if self.meta:
            json_span['meta'] = self.meta

        return json.dumps(json_span)

    def close(self):
        # Close the span when not done manually
        if not self.end and not self.duration and not self.is_annotation():
            self.end = time.time()

    def is_annotation(self):
        """Check if that span should have no duration"""
        return self.type == "annotation"

    def create_child(self, span_type=None):
        child = Span(trace_id=self.trace_id, parent_id=self.span_id, start=time.time())
        if span_type:
            child.type = span_type

        return child

    def annotate(self, message, meta=None):
        annotation_span = self.create_child(span_type="annotation")
        annotation_span.meta = {
            "message": message
        }
        if meta:
            annotation_span.meta.update(meta)

        return annotation_span


def new_span(span_type=None):
    return Span(trace_id=new_trace_id(), span_id=new_span_id(), start=time.time(), span_type=span_type)


def new_trace_id():
    return random.getrandbits(64)


def new_span_id():
    return random.getrandbits(32)
