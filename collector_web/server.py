from datetime import datetime as dt
import os
import json

from flask import Flask, render_template, send_from_directory
import sqlite3

app = Flask(__name__)


def epoch_to_utc_dt(s):
    return dt.utcfromtimestamp(float(s))

@app.route('/js/<path:path>')
def send_js(path):
    app.logger.info(path)
    return send_from_directory('js', path)

@app.route('/css/<path:path>')
def send_css(path):
    return send_from_directory('css', path)

@app.route("/")
def trace_list():
    db = get_db()
    cur = db.cursor()
    cur.execute("SELECT * FROM span WHERE parent_id = 0")
    rows = cur.fetchall()

    app.logger.info(len(rows))

    traces = []
    for row in rows:
        row['start_dt'] = epoch_to_utc_dt(row['start'])
        traces.append(row)

    return render_template("list.html", traces=traces)


@app.route("/<int:trace_id>")
def trace_show(trace_id):
    db = get_db()
    cur = db.cursor()
    app.logger.info(trace_id)
    cur.execute("SELECT * FROM span WHERE trace_id = ?", (trace_id, ))
    rows = cur.fetchall()

    parent = None
    children = []
    for row in rows:
        if row["parent_id"]:
            children.append(row)
        else:
            parent = row

    return render_template(
        "show.html",
        parent=json.dumps(parent, sort_keys=True, indent=4, separators=(',', ': ')),
        children=json.dumps(children, sort_keys=True, indent=4, separators=(',', ': ')),
    )


def dict_factory(cursor, row):
    d = {}
    for idx, col in enumerate(cursor.description):
        d[col[0]] = row[idx]
        if col[0] == "json_meta":
            d[col[0]] = json.loads(unicode(d[col[0]]))
    return d


def get_db():
    db_file = os.path.join(os.path.dirname(os.path.realpath(__file__)), '../db.sqlite3')
    conn = sqlite3.connect(db_file)
    conn.row_factory = dict_factory
    return conn


if __name__ == "__main__":
    app.run(
        debug=True,
        static_files={
             '/css': os.path.join(os.path.dirname(__file__), 'static/css'),
             '/js': os.path.join(os.path.dirname(__file__), 'static/js')
        }
    )
