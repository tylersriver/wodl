# WODL - Claude Code Instructions

## Project Overview
Workout logging web app. Tracks lifts (with 1RM calculator + percentage tables) and CrossFit-style WODs.

## Tech Stack
- Go backend with chi router, SQLite (modernc.org/sqlite pure Go driver)
- DaisyUI + Tailwind CSS + HTMX frontend (CDN, no build step)
- JWT auth (HTTP-only cookies), bcrypt passwords

## Architecture
- Strict DDD: domain -> application -> infrastructure -> interface layers
- CQRS: commands for writes, queries for reads
- Domain entities use Validated wrappers and factory constructors (NewUser, NewLift, etc.)
- Repository interfaces in domain layer, SQLite implementations in infrastructure
- Manual DI wiring in cmd/wodl/main.go

## Key Commands
```bash
go build ./cmd/wodl     # Build
go test ./...           # Run all tests
go vet ./...            # Lint
PORT=8080 ./wodl        # Run (also reads JWT_SECRET, DB_PATH env vars)
docker compose up       # Run with Docker
```

## Conventions
- Soft deletes on Lift and Workout entities
- 1RM uses Epley formula: weight * (1 + reps/30)
- Templates are embedded via Go embed (internal/interface/web/templates/embed.go)
- DB migrations are inline SQL in infrastructure/db/sqlite/db.go (embed can't use `..` paths)
- No build step for frontend — all CSS/JS via CDN
