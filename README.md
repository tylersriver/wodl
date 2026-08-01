# WODL - Workout Logger

A workout logging app for tracking lifts and CrossFit-style WODs. Log your sessions, calculate estimated 1RM, and view percentage tables.

## Features

- **Workout of the day** - Plan a session for a given day; the app opens on whatever is assigned to today
- **Import from a photo** - Upload screenshots of your gym's programming and Claude fills in the session for you to check before it's saved (optional; needs `ANTHROPIC_API_KEY`)
- **Lift tracking** - Log weight, reps, sets with automatic 1RM estimation (Epley formula) and percentage tables
- **WOD tracking** - Log AMRAP, For Time, EMOM, Tabata, Chipper, and custom workouts
- **Results** - Lifts and workouts in one filterable list
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
| `GROQ_API_KEY` | _(unset)_ | Enables importing a session from photos, via Groq. Takes precedence over `ANTHROPIC_API_KEY`. |
| `GROQ_MODEL` | `meta-llama/llama-4-scout-17b-16e-instruct` | Vision model to use with Groq. Override if that model isn't available to your key. |
| `GROQ_MAX_IMAGE_EDGE` | `1024` | Longest edge, in pixels, that an uploaded image is scaled to before sending. Vision models bill by pixel area, so lower this if you hit a tokens-per-minute limit; `0` disables scaling. |
| `ANTHROPIC_API_KEY` | _(unset)_ | Enables importing a session from photos, via Claude. Used when `GROQ_API_KEY` is unset. |

With neither key set, image import is hidden and the rest of the app is unaffected.

To see which models a Groq key can reach:

```bash
curl https://api.groq.com/openai/v1/models -H "Authorization: Bearer $GROQ_API_KEY"
```

Groq's free tier allows 8,000 tokens per minute, and images dominate that budget.
Two full-resolution phone screenshots exceed it on their own, which is why uploads
are scaled down before sending. If an import still reports "request too large",
lower `GROQ_MAX_IMAGE_EDGE` (try `800`) or upload one image at a time.

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
