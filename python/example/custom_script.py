"""Example of an intrumented custom script"""

import time

from raclette.client import Client
from raclette.span import new_span


Client.init()
span = new_span()
span.type = "Custom script"

# Here, let's do our business
print "Doing the job..."
time.sleep(.18)

# Let's add application-specific meta
span.add_meta("example.job", "example.py")
span.add_meta("example.weather", "Sunny")

print "Good, let's take a break. And don't forget to report it!"
span.annotate(
    message="Taking a break of 1 second",
    meta={"example.break.reason": "Can't work too much", "example.break.allowed": "true"}
)
time.sleep(1)
span.annotate(
    message="We took a break, feeling better now!",
    meta={"example.break.duration": "1s"}
)

print "Back to work..."
span.add_meta("animal.fox.say", "ding ding ding ding ding ding ding")

http_call_span = span.create_child(span_type="http")
http_call_span.duration = 0.1
http_call_span.meta = {
	"http.response_code": str(200),
	"http.url": "google.com",
	"http.response_size": str(4512),
}

time.sleep(.05)

print "Boom, job is done! Report spans."
Client.flush()

print "Spans reported, job is over."
