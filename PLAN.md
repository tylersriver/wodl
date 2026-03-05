# WODL - Workout Logger Application Design Plan

## Implementation Progress

### Phase 1: Project scaffold + domain layer
- [ ] Initialize Go module, install dependencies
- [ ] Create directory structure
- [ ] Implement domain entities with validation (User, Lift, LiftLog, Workout, WorkoutResult)
- [ ] Implement 1RM calculator
- [ ] Define repository interfaces
- [ ] Write unit tests for all domain logic

### Phase 2: Infrastructure + database
- [ ] Create SQLite migrations
- [ ] Implement SQLite repositories (all 5)
- [ ] Implement auth infrastructure (JWT, bcrypt)
- [ ] Write repository integration tests (using in-memory SQLite)

### Phase 3: Application layer
- [ ] Define commands, queries, result types
- [ ] Implement services (AuthService, LiftService, WorkoutService)
- [ ] Write service unit tests with mocked repos

### Phase 4: Interface layer + frontend
- [ ] Set up chi router and middleware (auth, logging)
- [ ] Create base template layout with DaisyUI
- [ ] Implement auth handlers + pages (login, register)
- [ ] Implement lift handlers + pages (list, detail, log)
- [ ] Implement workout handlers + pages (list, detail, log)
- [ ] Add HTMX interactions (inline 1RM calc, dynamic form updates)

### Phase 5: E2E tests
- [ ] Test helper to spin up full app with in-memory SQLite
- [ ] E2E tests for auth flows
- [ ] E2E tests for lift logging + 1RM
- [ ] E2E tests for workout logging

---

## Architecture

```
cmd/wodl/main.go                    # Entry point, DI wiring
internal/
├── domain/
│   ├── entities/                   # Core business objects + validation
│   └── repositories/              # Repository interfaces
├── application/
│   ├── services/                  # Application services
│   ├── command/                   # Write operations (CQRS)
│   ├── query/                     # Read operations (CQRS)
│   ├── common/                    # Result DTOs
│   ├── interfaces/                # Service interfaces
│   └── mapper/                    # Layer mappers
├── infrastructure/
│   ├── db/
│   │   └── sqlite/               # SQLite repository implementations
│   ├── auth/                     # Password hashing, JWT
│   └── middleware/               # Auth middleware
├── interface/
│   ├── web/                      # HTTP handlers (controllers)
│   │   ├── handlers/             # Route handlers
│   │   ├── templates/            # Go HTML templates + DaisyUI
│   │   └── static/               # CSS/JS assets
│   └── dto/                      # Request/response structs
├── testhelpers/                  # Shared test utilities
migrations/                       # SQLite migrations
```

## Domain Model

### Entities
- **User**: Id, Email, PasswordHash, DisplayName, CreatedAt, UpdatedAt
- **Lift**: Id, UserId, Name, Category, OneRepMax, CreatedAt, UpdatedAt
- **LiftLog**: Id, UserId, LiftId, Weight, Reps, Sets, RPE, Estimated1RM, PercentOf1RM, Notes, LoggedAt, CreatedAt
- **Workout**: Id, UserId, Name, Type (AMRAP/ForTime/EMOM/Tabata/Chipper/Custom), Description, TimeCap, Rounds, Interval, CreatedAt, UpdatedAt
- **WorkoutResult**: Id, UserId, WorkoutId, Score, ScoreType, Rx, Notes, LoggedAt, CreatedAt

### 1RM Calculator
- Epley formula: `weight * (1 + reps/30.0)`
- Percentage table: 50-100% in 5% increments

## Tech Stack
- Go 1.22+ with chi router
- SQLite via modernc.org/sqlite (pure Go)
- Go html/template + DaisyUI (CDN) + HTMX (CDN)
- JWT (HTTP-only cookie) + bcrypt for auth
- golang-migrate for migrations
- testify for testing
