"""Example of an intrumented custom script"""

import time

from raclette.client import Client
from raclette.span import new_span


Client.init()
span = new_span()
span.service = "example"
span.type = "custom"

# Here, let's do our business
print "Doing the job..."
time.sleep(.18)

# Let's add application-specific meta
span.add_meta("example.job", "example.py")
span.add_meta("example.weather", "Sunny")

print "Good, let's take a break. And don't forget to report it!"
span.create_child(
    span_type="note",
    resource="Taking a break!",
    meta={"example.break.reason": "Can't work too much", "example.break.allowed": "true"},
)

time.sleep(1)
span.create_child(
    span_type="note",
    resource="We took a break, feeling better now!",
    meta={"example.break.duration": "1s"},
)

print "Back to work..."
span.add_meta("animal.fox.say", "ding ding ding ding ding ding ding")

span.create_child(
    span_type="http",
    duration=0.1,
    resource="https://api.ipify.org?format=json",
    meta={
        "http.response_code": str(200),
        "http.url": "google.com",
        "http.response": '{"ip":"177.193.147.116"}',
        "http.response_size": str(24),
    },
)

time.sleep(.05)

print "Boom, job is done! Report spans."
Client.flush()

print "Spans reported, job is over."
