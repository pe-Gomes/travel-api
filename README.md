# Travel API / plann.er

## Before running the project

The project uses a _PostgreSQL_ database, so you need to create
a database and set the environment variables in the env script.

For testing emails you can use _Mailtip_ and the front end
will run on port 8025, while the back end will run on port `1025`.

By default, the server will run on port `8080`.

````bash

```bash
source env.zsh
docker compose up -d --build
````

## Generating API Code with GoAPI Gen

```bash
goapi-gen --out  ./internal/api/spec/travel.gen.spec.go ./internal/api/spec/travel.spec.json
```

## Generating SQL Queries with sqlc and migrations with tern

```bash
# Initialize migrations folder
tern init ./internal/pgstore/migrations

# Make a new migration
tern new --migrations ./internal/pgstore/migrations {NAME}

# Apply migrations
tern migrate --migrations ./internal/pgstore/migrations --config ./internal/pgstore/migrations/tern.conf
# or to apply all commands
go generate ./...
```
