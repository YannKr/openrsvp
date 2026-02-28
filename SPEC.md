# OpenRSVP — Implementation Plan

## Context

Building an open-source, self-hosted alternative to Evite focused on privacy, simplicity, and ad-free experience — primarily for kids birthday party invitations. Data is auto-deleted after a configurable retention period (default 30 days post-event). The project ships as a single Docker container with SQLite by default and optional PostgreSQL support.

## Tech Stack

| Layer | Choice |
|-------|--------|
| Backend | Go 1.23+ with chi router |
| Frontend | SvelteKit + Tailwind CSS |
| Database | SQLite (default) / PostgreSQL (option) |
| Auth | Magic link (passwordless) |
| Notifications | Pluggable providers (SMTP, SendGrid, SES for email; Twilio, Vonage for SMS) |
| Bot defense | Honeypot fields + IP-based rate limiting |
| Deployment | Multi-stage Docker build → single binary with embedded frontend |

## Directory Structure

```
openrsvp/
├── cmd/openrsvp/main.go              # Entry point, DI wiring
├── internal/
│   ├── config/config.go              # Env-based config
│   ├── database/
│   │   ├── database.go               # DB interface + factory
│   │   ├── sqlite.go, postgres.go    # Driver implementations
│   │   └── migrations/*.sql          # Embedded SQL migrations
│   ├── auth/                         # Magic link auth + middleware
│   ├── event/                        # Event CRUD (model, store, service, handler)
│   ├── rsvp/                         # RSVP system (model, store, service, handler)
│   ├── invite/                       # Invite card templates + customization
│   ├── message/                      # Organizer-attendee messaging
│   ├── notification/
│   │   ├── provider.go               # Provider interface
│   │   ├── service.go                # Dispatch + routing
│   │   ├── email/ (smtp, sendgrid, ses)
│   │   ├── sms/ (twilio, vonage)
│   │   └── templates/*.html          # Email templates (go:embed)
│   ├── scheduler/                    # Background jobs (reminders, cleanup)
│   ├── security/                     # Rate limiter, honeypot, CSRF, sanitization
│   └── server/ (server.go, router.go)
├── web/                              # SvelteKit frontend
│   └── src/
│       ├── lib/
│       │   ├── api/                  # Typed fetch wrappers
│       │   ├── components/           # UI, invite, rsvp, event, message, layout
│       │   ├── stores/               # auth, events, toast
│       │   ├── types/                # TypeScript types
│       │   └── utils/                # validation, dates, honeypot
│       └── routes/
│           ├── auth/ (login, verify, logout)
│           ├── events/ (list, new, [eventId]/{edit,invite,messages,share})
│           ├── i/[token]/            # Public invite + RSVP
│           └── r/[token]/            # RSVP modification
├── Dockerfile                        # 3-stage: Node → Go → Alpine
├── docker-compose.yml                # SQLite mode
├── docker-compose.postgres.yml       # PostgreSQL override
├── Makefile, .env.example, LICENSE, README.md
└── .github/workflows/ (ci.yml, release.yml)
```

## Database Schema (9 tables)

- **organizers** — id, email, name, timestamps
- **magic_links** — token_hash, organizer_id, expires_at, used_at
- **sessions** — token_hash, organizer_id, expires_at
- **events** — organizer_id, title, description, event_date, location, timezone, retention_days, status (draft/published/cancelled/archived), share_token
- **invite_cards** — event_id, template_id, heading/body/footer text, colors, font, custom_data JSON
- **attendees** — event_id, name, email/phone, rsvp_status (pending/attending/maybe/declined), rsvp_token, contact_method
- **messages** — event_id, sender_type/id, recipient_type/id, subject, body
- **reminders** — event_id, remind_at, target_group, status (scheduled/sent/cancelled/failed)
- **notification_log** — event_id, attendee_id, channel, provider, status, error

IDs are UUIDv7 stored as TEXT. Share tokens are 8-char base62. RSVP tokens are 12-char base62.

## API Endpoints (30 total, all under `/api/v1`)

**Auth (4):** magic-link request, verify, logout, me
**Events (7):** CRUD + publish + cancel
**Invite Cards (4):** list templates, get/save invite, preview
**Public RSVP (4):** get invite by share token, submit RSVP, get/update RSVP by token
**RSVP Management (3):** list RSVPs, stats, remove attendee
**Messaging (4):** organizer send/list, attendee send/list
**Reminders (4):** CRUD for scheduled reminders
**Health (2):** health + readiness

---

## Phase 1: Project Foundation (L)

Set up repository, build system, database layer, SvelteKit skeleton, and Docker.

**Files:** `go.mod`, `cmd/openrsvp/main.go`, `internal/config/config.go`, `internal/database/*`, `internal/server/*`, `web/` (SvelteKit scaffold), `Dockerfile`, `docker-compose.yml`, `Makefile`, `.env.example`, `.github/workflows/ci.yml`

**Key decisions:**
- `database/sql` as abstraction; `Dialect()` method for SQL differences (placeholder `?` vs `$1`)
- SQLite: WAL mode + foreign keys enabled at connection
- Migrations embedded via `go:embed`
- SvelteKit `adapter-static` → Go serves static files via `go:embed`
- Vite dev server proxies `/api` to Go backend

**Go deps:** chi, cors, godotenv, go-sqlite3, lib/pq, golang-migrate, zerolog, uuid
**Node deps:** svelte, sveltekit, adapter-static, tailwindcss, typescript, eslint, prettier

**Verification:** `make build` compiles Go binary, `cd web && npm run build` builds frontend, `docker build .` produces image, `docker run` starts server on :8080 with health check passing.

---

## Phase 2: Core Backend (L)

Implement auth, event CRUD, RSVP, invite cards, and the notification provider interface.

### 2.1 Auth System
- Magic link: 32-byte crypto-random token, SHA-256 hashed, 15-min expiry, single-use
- Sessions: 7-day expiry, cookie + Bearer token support
- Auto-create organizer on first magic link request
- Middleware injects organizer into request context

### 2.2 Event CRUD
- Status machine: draft → published ↔ draft, published → cancelled, any → archived
- Share token: 8-char base62 for friendly URLs (`/i/Ab3kX9pQ`)
- Soft delete for retention window; hard delete by cleanup job

### 2.3 RSVP System
- Duplicate detection by email/phone per event (upsert)
- 12-char base62 RSVP token for attendee identification
- Stats endpoint: `{ attending, maybe, declined, pending, total }`

### 2.4 Invite Card System
- 5 templates defined in code: `balloon-party`, `confetti`, `unicorn-magic`, `superhero`, `garden-picnic`
- Customization (colors, text, font) stored in DB; rendering is client-side in Svelte

### 2.5 Notification Provider Interface
- `Provider` interface: `Name()`, `Channel()`, `Send()`, `SendBatch()`, `HealthCheck()`
- `ProviderRegistry` manages active providers per channel
- SMTP provider implemented first (others in Phase 4)
- Go `html/template` for email templates

**Verification:** All 30 API endpoints respond correctly via `curl`/Postman. Auth flow works end-to-end. RSVP creates attendee records.

---

## Phase 3: Frontend — Core Pages (L)

Build all Svelte pages and components for the complete user experience.

### 3.1 Design System
- Custom UI components (Button, Input, Card, Modal, Toast, Badge, Spinner, DateTimePicker) — all built with Tailwind, no component library
- Color palette: primary indigo `#6366f1`, secondary pink `#f0abfc`, slate grays, white backgrounds
- WCAG 2.1 AA: focus rings, aria labels, keyboard nav, sufficient contrast

### 3.2 Pages
- **Landing:** Hero + features grid + CTA (static, no API calls)
- **Auth:** Magic link request → check email → verify token → redirect
- **Event Creation:** 3-step wizard (details → description/settings → review)
- **Invite Designer:** Template picker + editor on left, live preview on right
- **Public Invite (`/i/:token`):** Beautiful card + RSVP form with honeypot
- **RSVP Modification (`/r/:token`):** View/change RSVP + message organizer
- **Dashboard:** Event list as cards, event detail with RSVP stats (number cards), attendee list with filter tabs, share page

**Verification:** Full flow works in browser: create event → design invite → share link → RSVP as attendee → organizer sees RSVP on dashboard.

---

## Phase 4: Notifications & Messaging (M)

### 4.1 Additional Providers
- SendGrid (raw HTTP, no SDK), SES (AWS SDK v2), Twilio (raw HTTP), Vonage (raw HTTP)
- Provider selection via env var: `NOTIFICATION_EMAIL_PROVIDER=sendgrid`

### 4.2 Background Scheduler
- In-process worker pool using goroutines (no external queue)
- Reminder job: polls every 30s, sends notifications, updates status
- Cleanup job: polls every 1h, purges expired event data
- Row-level locking to prevent duplicate processing

### 4.3 Messaging System
- Organizer → group (all/attending/maybe/declined) or individual
- Attendee → organizer
- Messages trigger notifications via configured provider

### 4.4 Reminder Management
- Schedule reminders with date/time, target group, optional custom message
- Frontend: simple form on event detail page

**Verification:** Schedule a reminder → wait for trigger time → notification sent. Send message from organizer → attendee receives notification. Send message from attendee → organizer sees it.

---

## Phase 5: Security & Data Lifecycle (M)

### 5.1 Rate Limiting
- In-memory sliding window (sync.Map) with DB-backed option for multi-instance
- Limits: 10/min auth, 30/min RSVP, 100/min general API
- Returns `429` with `Retry-After` header

### 5.2 Honeypot
- Hidden field named `website` — CSS hidden, aria-hidden, tabindex=-1
- If filled: return fake 200 success, discard RSVP (bot doesn't know it failed)

### 5.3 Input Sanitization
- `bluemonday` for HTML sanitization (limited tags in message bodies)
- E.164 phone validation, email regex + optional MX lookup

### 5.4 CSRF Protection
- Synchronizer Token Pattern: CSRF token in cookie, validated via `X-CSRF-Token` header
- Public RSVP endpoints rely on honeypot + rate limiting instead

### 5.5 Data Retention
- Cleanup job hard-deletes events where `event_date + retention_days < now()`
- Cascade deletes all related records in a single transaction
- Warning notification 7 days before auto-deletion
- Organizer can extend retention or trigger immediate deletion

**Verification:** Exceed rate limit → get 429. Submit RSVP with honeypot filled → silent discard. Create event with 1-day retention → data auto-deleted after expiry.

---

## Phase 6: Polish & Deployment (M)

### 6.1 Docker Optimization
- 3-stage build with cache mounts → target image < 30MB
- `HEALTHCHECK` instruction in Dockerfile
- `docker-compose.postgres.yml` adds Postgres 16 service

### 6.2 Production Readiness
- Graceful shutdown: stop accepting connections → drain (30s timeout) → stop scheduler → close DB
- Health endpoints: `/health` (simple), `/health/ready` (DB check)
- SPA fallback: unmatched routes serve `index.html`

### 6.3 CI/CD
- CI: matrix Go 1.23/1.24, test with SQLite + Postgres, lint, build Docker
- Release: on `v*` tag → multi-platform images (amd64/arm64) → GHCR + Docker Hub

### 6.4 Documentation
- README with screenshots and quick start
- Self-hosting guide (Docker, reverse proxy, SQLite vs Postgres, backups)
- API docs, developer guide, CONTRIBUTING.md

**Verification:** `docker compose up` starts the app. Health check passes. Full user flow works in the container. CI pipeline passes on GitHub.

---

## Implementation Strategy

Each phase will be implemented by parallel agents where possible:
- **Phase 1:** 3 agents — Go scaffold + DB layer, SvelteKit scaffold, Docker + CI
- **Phase 2:** 3 agents — Auth system, Event + RSVP + Invite APIs, Notification interface
- **Phase 3:** 3 agents — Design system + layout, Core pages (auth, events, invite designer), Public pages (invite, RSVP, dashboard)
- **Phase 4:** 2 agents — Notification providers + scheduler, Messaging system + reminders
- **Phase 5:** 2 agents — Security middleware (rate limit, honeypot, CSRF, sanitization), Data retention cleanup
- **Phase 6:** 2 agents — Docker + CI/CD, Documentation

Phases are sequential (each depends on the previous), but work within each phase is parallelized.
