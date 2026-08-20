-- Seeded once on the first postgres volume init (docker-entrypoint-initdb.d).
CREATE TABLE IF NOT EXISTS items (
    id         serial PRIMARY KEY,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO items (name) VALUES
    ('first seed item'),
    ('second seed item'),
    ('third seed item');
