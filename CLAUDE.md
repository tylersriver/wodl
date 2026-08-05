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
- `/` takes `?date=YYYY-MM-DD` to show any day. The arrows, the swipe gesture and the
  create-session date default all key off that one parameter, so day-stepping degrades
  to plain links without JavaScript. An unparseable date falls back to today
- A session can be built from photos of a gym's programming at `/import`. Extraction
  and creation are separate steps on purpose: `/import/extract` only reads and renders
  an editable review, and nothing is written until the user posts it back.
- A session date is a **civil date**, not an instant: "the plan for Sunday" is the same
  day for whoever reads it. Every one is anchored at midnight UTC — see
  `handlers/civil_date.go`, and go through its helpers rather than `time.Now()` or
  `time.Local`. Which day is *today*, though, is the reader's question: the server clock
  is UTC, so before this the landing page rolled over to tomorrow at 6pm Mountain. The
  browser reports its IANA zone in a `tz` cookie (`timezone_script`, on every page
  including log-in) and `requestLocation` resolves the day in it, falling back to the
  server's zone when the cookie is missing or unusable. Two consequences worth knowing:
  the cookie value must go in **unencoded**, because Go hands cookie values back verbatim
  and `America%2FDenver` is not a zone anyone can look up; and `handlers` imports
  `time/tzdata`, because the Alpine image carries no zoneinfo and every lookup would
  otherwise fail into UTC — exactly the bug being fixed. `LoggedAt` and friends *are*
  instants, so they render through the `when` helper with the `Loc` the handler passes down

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
- Reference cached assets through `{{asset "app.css"}}` (`static.AssetURL`), never a bare
  path. Pages are network-first while `/static/` is cached by both the service worker and a
  day of `max-age`, so a bare path pairs a new build's markup with the old stylesheet — and
  since Tailwind only emits classes it finds, a class added alongside its markup is missing
  outright and the element renders with *no* styling. The content hash makes the URL
  uncacheable across builds. `sw.js` therefore pre-caches only the unhashed assets
- One template set, built by `templates.Must()`. The server and the e2e harness both call
  it; they used to each build their own FuncMap and drifted, so a helper added for a page
  panicked in whichever one was forgotten
- static/app.css is a BUILD ARTIFACT that is committed. Tailwind only emits classes it finds
  in the templates, so after editing markup you must re-run `cd frontend && npm run build`
  or new utility classes silently won't exist. Go itself needs no frontend toolchain.
- UI components come from Basecoat: semantic markup (`.btn`, `.field > label + input`, `.card`)
  driven by `data-variant` / `data-size` attributes rather than class soup
- The look is one dark theme — warm charcoal, soft cards, a single mint accent, numbers set
  large. It lives entirely in `frontend/input.css`: Basecoat reads its colours from the shadcn
  custom properties, so re-pointing `--background`/`--primary`/`--radius`/… on `:root, .dark`
  re-skins every button, badge, input and dialog at once. **Never hardcode a colour in a
  template** — reach for `bg-card`, `text-primary`, `border-primary/40` and the palette
  follows. `<html class="dark">` is on every page *including log-in and register*, because
  Basecoat's own `dark:` variants key off `html.dark`; without it its components disagree
  with the palette. `--chevron-down-icon` and `--check-icon` must be restated alongside the
  palette: Basecoat bakes the light theme's grey into those SVG data URIs, so an unrecoloured
  chevron is invisible on charcoal
- Three house component classes carry the look, all in `@layer components` and all
  deliberately colourless so a utility picks the colour: `.eyebrow` (the tracked-out capital
  label above a heading — mint when it names the thing you came for, muted when it names
  context), `.numeral` (anything read as a quantity: display face, tabular figures, tight
  tracking), and `.stat-tile` (one cell of a percentage ladder; `data-state="on"` marks 100%)
- DM Sans (body) and Archivo (display, via the `font-display` utility) are self-hosted woff2
  in `static/`. They are *variable* fonts, so one file per unicode range covers every weight —
  don't add per-weight files. app.css names them by `/static/…` path rather than through
  `asset`: the stylesheet asking for them already carries a content hash, and a different font
  would arrive under a different name. New font files must be added to the `//go:embed` list
  in static/embed.go, and `sw.js` pre-caches the latin ones so a cold offline launch doesn't
  paint in the system face and reflow
- Enums reach templates snake_case (`for_time`, `rounds_and_reps`). Render them through the
  `label` helper, which swaps the underscores for spaces — the stored value is unchanged, and
  what was tolerable inside a lowercase badge is not once the same string is set as an
  `.eyebrow`
- Modals are native `<dialog class="dialog sheet">` opened with `openDialog(id)`; `sheet` makes
  them full-width bottom sheets on phones
- Session cards (`session_workout_card`, `session_warmup_card`) fold: a native `<details>`
  that starts open, so the page still works without JavaScript and nothing is hidden until
  you hide it. The fold is remembered per card in `sessionStorage` under `data-card-key`,
  which is what keeps the steps you're done with folded after a round-trip to log a score;
  the tab-scoped store is deliberate, since that state is about the session you're in the
  middle of. They use plain `bg-card rounded-2xl border` rather than Basecoat's `.card` —
  `.card` is unlayered and sets its own `padding-block` that no utility can turn off, which
  would stop the summary running the full height of its tap target
- The installed app is `black-translucent` with `viewport-fit=cover`, so the web view
  runs the full height of the screen and the status bar floats over it. That is what lets
  the charcoal reach the edges rather than sit under a system-coloured strip, and the cost
  is that the page must inset itself: the sticky header pads by `env(safe-area-inset-top)`
  (padding the header, not the body, so the charcoal sits *behind* the status bar and the
  bar stays pinned when the page scrolls), and log-in/register pad their own body. Without
  it the wordmark renders under the Dynamic Island. The style is per-document, so every
  page that has its own `<head>` has to state it or the bar flips back
- Mobile nav is the bottom tab bar in layout.html (Today / Results / Sessions); its active
  item is set client-side from `location.pathname`, so handlers don't pass down a
  "current page" flag
- The bar is `fixed`, so `<main>` reserves room for it with `.has-tabbar`. Never put a
  padding-bottom utility (`py-6`, `pb-*`) on that element: utilities sit in a later
  cascade layer than components, so they silently beat `.has-tabbar` and drop the page's
  last control under the bar — which is how Save/Cancel ended up unreachable on
  `/import/extract`. The shell sets `pt-8` only
- Basecoat's own rules are **unlayered**, so they outrank every Tailwind utility, not just
  component-layer CSS. `sr-only` is the known casualty: Basecoat ships `.sr-only{width:auto}`
  which beats Tailwind's layered utility, leaving the element 13px wide instead of 1px.
  Don't reach for `sr-only` to hide a form control — prefer `aria-label` on the control
  itself. When a utility mysteriously doesn't apply, check for a Basecoat rule first
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
