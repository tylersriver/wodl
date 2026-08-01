# WODL - Workout Logger

A workout logging app for tracking lifts and CrossFit-style WODs. Log your sessions, calculate estimated 1RM, and view percentage tables.

## Features

- **Lift tracking** - Log weight, reps, sets with automatic 1RM estimation (Epley formula) and percentage tables
- **WOD tracking** - Log AMRAP, For Time, EMOM, Tabata, Chipper, and custom workouts
- **Search** - Find lifts and workouts from the dashboard
- **Auth** - User accounts with JWT authentication

## Quick Start

### Docker

```bash
docker compose up
```

Open http://localhost:8080, register an account, and start logging.

### Local

Requires Go 1.25+.

```bash
go build ./cmd/wodl
PORT=8080 ./wodl
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `JWT_SECRET` | `wodl-dev-secret-change-in-production` | JWT signing key |
| `DB_PATH` | `wodl.db` | SQLite database file path |

## Development

```bash
go test ./...    # Run tests
go vet ./...     # Lint
```

### Styles

The stylesheet is compiled from Tailwind CSS v4 + [Basecoat](https://basecoatui.com)
into `internal/interface/web/static/app.css`, which is committed and embedded in the
binary — building or running the app needs no Node toolchain.

Rebuild it after changing any template, since Tailwind only emits the classes it finds
in the markup:

```bash
cd frontend
npm install
npm run build    # or: npm run watch
```

## Tech Stack

- **Backend**: Go, chi router, SQLite (pure Go driver)
- **Frontend**: Server-rendered HTML, Basecoat (shadcn/ui without React), Tailwind CSS v4, HTMX — self-hosted, no CDNs
- **Auth**: JWT (HTTP-only cookies), bcrypt
- **Architecture**: Domain-Driven Design, CQRS
