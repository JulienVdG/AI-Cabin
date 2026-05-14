---
name: sqlc-queries
description: Type-safe SQL queries with sqlc [schema, queries, generated types, PostgreSQL, MySQL, database, migration]
license: MIT
compatibility: opencode
metadata:
  tool: sqlc
---

## What I do

I enforce sqlc patterns for type-safe database queries.

## Module Structure

```
db/
├── schema/      # Database schema (.sql)
├── queries/     # SQL queries (.sql)
├── models.go    # Generated types
├── db.go        # Database connection
└── queries.sqlc.go  # Generated query methods
```

## Configuration

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"  # or "mysql"
    queries: "db/queries/"
    schema: "db/schema/"
    gen:
      go:
        package: "db"
        out: "db"
```

## Schema Files

```sql
-- db/schema/users.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    title VARCHAR(255) NOT NULL,
    content TEXT,
    published BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

## Query Files

```sql
-- db/queries/users.sql
-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetAllUsers :many
SELECT * FROM users
ORDER BY created_at DESC;

-- name: CreateUser :one
INSERT INTO users (email, name)
VALUES ($1, $2)
RETURNING *;

-- db/queries/posts.sql
-- name: GetPublishedPosts :many
SELECT * FROM posts
WHERE published = true
ORDER BY created_at DESC;

-- name: GetPostsByUser :many
SELECT * FROM posts
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetPostByID :one
SELECT * FROM posts
WHERE id = $1;
```

## Generated Code

```go
// db/models.go
type User struct {
    ID        int32
    Email     string
    Name      string
    CreatedAt time.Time
}

type Post struct {
    ID        int32
    UserID    int32
    Title     string
    Content   sql.NullString
    Published bool
    CreatedAt time.Time
}

// db/queries.sqlc.go
func (q *Queries) GetUserByID(ctx context.Context, id int32) (User, error)
func (q *Queries) GetAllUsers(ctx context.Context) ([]User, error)
func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
func (q *Queries) GetPublishedPosts(ctx context.Context) ([]Post, error)
func (q *Queries) GetPostsByUser(ctx context.Context, userID int32) ([]Post, error)
func (q *Queries) GetPostByID(ctx context.Context, id int32) (Post, error)
```

## Usage

```go
// main.go
queries := db.New(dbConn)

// Get all users
users, err := queries.GetAllUsers(ctx)
if err != nil {
    log.Fatal(err)
}

for _, user := range users {
    fmt.Println(user.Name)
}

// Get user by ID
user, err := queries.GetUserByID(ctx, 1)
if err != nil {
    log.Fatal(err)
}

// Create user
newUser, err := queries.CreateUser(ctx, db.CreateUserParams{
    Email: "alice@example.com",
    Name:  "Alice",
})
```

## Benefits

- ✅ Type-safe queries (compile-time errors)
- ✅ No ORM overhead
- ✅ Pure SQL (full control)
- ✅ Auto-generated models
- ✅ IDE autocomplete
- ✅ Database-agnostic (PostgreSQL, MySQL, SQLite)

## Commands

```bash
# Generate Go code from SQL
sqlc generate

# Or via Makefile
make generate
```

## Query Examples

### Joins

```sql
-- name: GetUserWithPosts :many
SELECT u.*, p.id as post_id, p.title as post_title
FROM users u
LEFT JOIN posts p ON u.id = p.user_id
WHERE u.id = $1
ORDER BY p.created_at DESC;
```

### Aggregations

```sql
-- name: GetPostCountByUser :many
SELECT u.id, u.name, COUNT(p.id) as post_count
FROM users u
LEFT JOIN posts p ON u.id = p.user_id
GROUP BY u.id, u.name
ORDER BY post_count DESC;
```

### Transactions

```go
tx := queries.db.BeginTx(ctx, nil)
qtx := queries.WithTx(tx)

err := qtx.CreateUser(ctx, db.CreateUserParams{...})
if err != nil {
    tx.Rollback()
    return err
}

err = qtx.CreatePost(ctx, db.CreatePostParams{...})
if err != nil {
    tx.Rollback()
    return err
}

err = tx.Commit()
```

## Related Skills

- `debug-go` - Go debugging patterns
