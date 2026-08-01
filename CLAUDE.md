# WODL - Claude Code Instructions

## Project Overview
Workout logging web app. Tracks lifts (with 1RM calculator + percentage tables) and CrossFit-style WODs.

## Core Model
- A **Session** is the plan assigned to one day — the workout of the day. It has a
  required date and an ordered list of workouts. It records what is *scheduled*, not
  that it was performed, so there is no session-completion entity.
- Results are recorded against the individual **Lift** (sets) and **Workout** (scores).
- The landing page (`/`) shows the session dated today; `/results` is the combined,
  filterable list of lifts and workouts.
- A session can be built from photos of a gym's programming at `/import`. Extraction
  and creation are separate steps on purpose: `/import/extract` only reads and renders
  an editable review, and nothing is written until the user posts it back.

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
PORT=8080 ./wodl        # Run (also reads JWT_SECRET, DB_PATH, ANTHROPIC_API_KEY env vars)
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
- Mobile nav is the bottom tab bar in layout.html (Today / Results / Sessions); its active
  item is set client-side from `location.pathname`, so handlers don't pass down a
  "current page" flag
- `/lifts` and `/workouts` 301 to `/results`; their detail pages are unchanged
- Schema changes go in `runVersionedMigrations` in infrastructure/db/sqlite/db.go, gated on
  `PRAGMA user_version`. Anything dropped there must also be removed from `migrationSQL`,
  which runs on every startup and would otherwise recreate it
- Image import picks its provider from whichever key is set: `GROQ_API_KEY` (with optional
  `GROQ_MODEL`) wins, else `ANTHROPIC_API_KEY`. With neither, the extractor is left nil,
  `ImportService.Enabled()` is false and the feature hides itself — always assign the port
  through a guard, since a typed nil stored in the interface would still test non-nil
- The vendor stays behind the `services.BoardExtractor` port; tests inject a stub via
  `testhelpers.NewTestAppWithExtractor`, so the import flow is covered without network access
- Anthropic states the shape with a tool schema; Groq states it in the prompt instead,
  because its vision models have not supported tools and images in the same request.
  Either way `decodeBoardPayload` clamps the result to the domain's enums — that shared
  clamp is what makes a loosely-schema'd provider safe to accept
- The Anthropic tool is deliberately **not** `Strict`. With strict decoding the model's own
  tool-call framing leaked into the payload: the workouts array arrived as literal text
  inside the preceding `warmup` string and `workouts` came back empty, so a legible board
  imported as "no workouts found". It reproduced on every strict run and none without.
  `TestExtractor_ToolIsNotStrict` pins it
- `GROQ_MODEL` exists because Groq's catalogue turns over quickly. List what a key can
  reach with `curl https://api.groq.com/openai/v1/models -H "Authorization: Bearer $GROQ_API_KEY"`
- Uploads are scaled to `GROQ_MAX_IMAGE_EDGE` (default 1024px) before sending. Vision
  models bill by pixel area and Groq's free tier is 8,000 tokens/minute, which two
  full-resolution screenshots exceed on their own. `shrinkToFit` leaves already-small
  images untouched rather than re-encoding them, and passes undecodable data through
- Groq sends one request per image, sequentially, and `mergeExtractions` recombines them.
  Groq rejects any single request larger than the whole per-minute budget, so batching
  images fails outright where splitting them fits; running them in parallel would spend
  the same minute's budget at once. The merge takes each field from the first image that
  had it and dedupes workouts by name, since boards reprint their header across screenshots
- Imported lifts and workouts are matched to existing ones by case-insensitive name so a
  repeated benchmark keeps one history; board wording (loads, "Score = Time", percentages
  of anything other than a 1RM) is kept verbatim in the description rather than reinterpreted
