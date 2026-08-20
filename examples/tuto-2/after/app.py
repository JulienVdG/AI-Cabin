import os

import psycopg2
import psycopg2.extras
from flask import Flask, abort, redirect, request

app = Flask(__name__)
DATABASE_URL = os.environ.get(
    "DATABASE_URL", "postgresql://app:app@postgres:5432/appdb"
)


def query_items():
    with psycopg2.connect(DATABASE_URL) as conn:
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute("SELECT id, name, created_at FROM items ORDER BY id")
            return cur.fetchall()


@app.get("/")
def index():
    rows = query_items()
    if rows:
        items_html = "".join(
            f"<li><strong>{r['name']}</strong> "
            f"<span>(#{r['id']}, {r['created_at']})</span></li>"
            for r in rows
        )
    else:
        items_html = "<li><em>No items yet. Insert one via psql or /add.</em></li>"
    return f"""<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Items</title></head>
<body>
<h1>Items</h1>
<ul>{items_html}</ul>
<form method="post" action="/add">
  <input name="name" placeholder="new item" />
  <button type="submit">Add</button>
</form>
</body>
</html>"""


@app.post("/add")
def add():
    name = request.form.get("name", "").strip()
    if not name:
        abort(400, "missing name")
    with psycopg2.connect(DATABASE_URL) as conn:
        with conn.cursor() as cur:
            cur.execute("INSERT INTO items (name) VALUES (%s)", (name,))
        conn.commit()
    return redirect("/", code=303)


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8000)
