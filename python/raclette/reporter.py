import httplib


class HttpReporter(object):

    @classmethod
    def send(cls, span):
        conn = httplib.HTTPConnection('localhost', 7777)
        conn.request("PUT", "/", span.serialize())
