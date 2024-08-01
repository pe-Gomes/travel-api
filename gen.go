package main

//go:generate goapi-gen --package=spec --out  ./internal/api/spec/travel.spec.go ./internal/api/spec/travel.spec.json
//go:generate tern migrate --migrations ./internal/pgstore/migrations --config ./internal/pgstore/migrations/tern.conf
//go:generate sqlc generate -f ./internal/pgstore/sqlc.yaml
