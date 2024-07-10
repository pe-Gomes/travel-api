# travel-api

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
# or
go generate ./...
```
