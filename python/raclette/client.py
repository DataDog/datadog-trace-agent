from .reporter import HttpReporter


# TODO: make a thread-safe client with global context

class Client(object):

    reporter = None
    spans = {}

    @classmethod
    def init(cls, config=None):
        cls.reporter = HttpReporter()

    @classmethod
    def add_span(cls, span):
        cls.spans[span.span_id] = span

    @classmethod
    def flush(cls):
        spans = cls.spans.values()
        cls.spans = {}

        for span in spans:
            span.close()
            cls.reporter.send(span)
