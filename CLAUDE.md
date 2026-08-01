# WODL - Claude Code Instructions

## Project Overview
Workout logging web app. Tracks lifts (with 1RM calculator + percentage tables) and CrossFit-style WODs.

## Tech Stack
- Go backend with chi router, SQLite (modernc.org/sqlite pure Go driver)
- Basecoat (shadcn/ui port, no React) + Tailwind CSS v4 + HTMX frontend, all self-hosted
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

cd frontend && npm install && npm run build   # Rebuild static/app.css after editing templates
cd frontend && npm run watch                  # Same, rebuilding on change
```

## Conventions
- Soft deletes on Lift and Workout entities
- 1RM uses Epley formula: weight * (1 + reps/30)
- Templates are embedded via Go embed (internal/interface/web/templates/embed.go)
- DB migrations are inline SQL in infrastructure/db/sqlite/db.go (embed can't use `..` paths)
- No CDNs — app.css and htmx.min.js are served from internal/interface/web/static so the PWA works offline
- static/app.css is a BUILD ARTIFACT that is committed. Tailwind only emits classes it finds
  in the templates, so after editing markup you must re-run `cd frontend && npm run build`
  or new utility classes silently won't exist. Go itself needs no frontend toolchain.
- UI components come from Basecoat: semantic markup (`.btn`, `.field > label + input`, `.card`)
  driven by `data-variant` / `data-size` attributes rather than class soup
- Modals are native `<dialog class="dialog sheet">` opened with `openDialog(id)`; `sheet` makes
  them full-width bottom sheets on phones
- Mobile nav is the bottom tab bar in layout.html; its active item is set client-side from
  `location.pathname`, so handlers don't pass down a "current page" flag
