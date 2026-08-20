# Tuto 2 — before

A minimal Flask app that lists rows from a Postgres table, plus a Postgres side
container seeded with a few items. This is the starting point: a project with an
existing Dockerfile and an existing compose, but **no cabin yet**.

## Run it

```bash
docker compose up --build
```

Then:

- http://localhost:8000 — lists the seeded items.
- Use the **Add** form — inserts a row through the web (a `POST /add` that
  redirects back, so refreshing does not re-insert).
- Insert straight into the database and watch it appear on the web:

  ```bash
  docker compose exec postgres psql -U app -d appdb \
      -c "INSERT INTO items (name) VALUES ('from psql');"
  ```

Then refresh http://localhost:8000.

`postgres` is also exposed on the host, loopback-only (not reachable from your
LAN), so you can reach it directly with `psql -h 127.0.0.1 -U app -d appdb`
(password `app`).

## What is here

| Path | Purpose |
|------|---------|
| `app.py` | Flask app: lists `items`, `/add` inserts |
| `requirements.txt` | `flask` + `psycopg2-binary` |
| `Dockerfile` | `python:3.12-slim` base, installs deps, runs the app |
| `db/init.sql` | Schema + seed, run once on first postgres init |
| `docker-compose.yml` | `web` + `postgres` services (postgres host-loopback-only) |
