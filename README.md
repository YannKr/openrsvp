# 🎉 OpenRSVP

A self-hosted, privacy-first alternative to Evite. Create beautiful event invitations, manage RSVPs, and communicate with guests — all without ads or data tracking. Perfect for birthday parties, gatherings, and celebrations.

## ✨ Features

- 🎨 **Beautiful Invitation Templates** — 5 customizable themes (Balloon Party, Confetti, Unicorn Magic, Superhero, Garden Picnic) with custom colors, fonts, and text
- 🔐 **Passwordless Auth** — Magic link sign-in, no passwords to manage
- 📋 **Easy RSVPs** — Guests respond with one click, no account needed. Track dietary needs and plus-ones
- 📬 **Notifications** — Pluggable email (SMTP, SendGrid, SES) and SMS (Twilio, Vonage, SNS) providers
- 💬 **Messaging** — Two-way communication between organizers and attendees
- ⏰ **Scheduled Reminders** — Automatic event reminders to guests
- 📝 **Guestbook** — Attendees can leave comments on event pages, delete their own, with organizer moderation
- 📥 **CSV Import** — Bulk import guest lists from CSV files with validation and duplicate detection
- 🔗 **Webhooks** — Real-time HTTP callbacks for RSVP and event lifecycle events with HMAC signing
- 📊 **Email Tracking** — Delivery status, open tracking, bounce/complaint handling, and per-event email statistics
- ✉️ **Unsubscribe & Suppression** — One-click unsubscribe footer on reminder/message emails, with a suppression list that skips opted-out addresses
- 🗣️ **Guest Feedback** — A "Report a problem" widget on public RSVP pages lets guests flag issues without logging in
- 🧭 **Setup Wizard** — First-run wizard for instance name, default timezone, sign-up policy, and support email
- 📦 **Data Export & Account Deletion** — Organizers can export all their data and permanently delete their account from an Account settings page
- 🛡️ **Privacy by Design** — Data auto-deletes after a configurable retention period (default 30 days post-event)
- 🤖 **Bot Protection** — Honeypot fields and IP-based rate limiting
- 📈 **Instance Admin** — Aggregate dashboard for instance-wide statistics (events, guests, RSVP rates, notification health, feature adoption) — privacy-first, no individual tracking
- 🏠 **Self-Hosted** — Single Docker container, you own your data
- 🗄️ **SQLite or PostgreSQL** — SQLite by default; PostgreSQL is fully supported and CI-tested. The full test suite runs against both

## 🚀 Quick Start

### Docker One-Liner

```bash
docker run -d -p 8080:8080 -v openrsvp-data:/data -e BASE_URL=http://localhost:8080 ghcr.io/yannkr/openrsvp:latest
```

Visit http://localhost:8080 and you're good to go! 🎊

### Docker Compose

```bash
git clone https://github.com/yannkr/openrsvp.git
cd openrsvp
cp .env.example .env
docker compose up -d
```

### With PostgreSQL

```bash
docker compose -f docker-compose.yml -f docker-compose.postgres.yml up -d
```

## 🛠️ Development

### Prerequisites

- Go 1.25+ (the `toolchain go1.26.3` directive auto-fetches the patched compiler when `GOTOOLCHAIN=auto`, the default)
- Node.js 22+
- Make

### Setup

```bash
# Install dependencies
go mod download
cd web && npm install && cd ..

# Copy environment config
cp .env.example .env

# Start the Go backend
make dev

# In another terminal, start the Svelte dev server
cd web && npm run dev
```

The Svelte dev server at http://localhost:5173 proxies API requests to the Go backend at http://localhost:8080.

### Build

```bash
# Build everything (frontend + backend)
make build

# Output: bin/openrsvp
```

### 🧪 Tests

```bash
make test   # Go unit/integration tests (SQLite; set TEST_DATABASE_URL for Postgres)
make lint   # golangci-lint
make e2e    # Playwright end-to-end suite
```

`make e2e` builds the binary, starts it on a scratch database on port 8091, runs
the suite against it, and tears it down again — no Docker required. Pass
`E2E_PORT` to use a different port, or a spec name to narrow the run
(`./scripts/e2e.sh app.spec.ts`). CI runs the same command as a blocking gate;
when it fails, the server log and Playwright traces are uploaded as the
`e2e-failure-artifacts` artifact.

To drive the suite against a server you started yourself, point it at the base
URL and at the server's log — the tests read magic link tokens out of it, which
the server only writes when `ENV=development`:

```bash
cd tests/e2e
E2E_BASE_URL=http://localhost:8091 E2E_SERVER_LOG=/path/to/server.log npx playwright test
```

With no `E2E_SERVER_LOG` set the tests fall back to `docker compose logs`, run
from the repository root (override the service name with `E2E_COMPOSE_SERVICE`).

### 🗄️ Database & Migrations

The full test suite runs against both SQLite and PostgreSQL in CI. Migrations live in per-dialect directories, `internal/database/migrations/sqlite` and `internal/database/migrations/postgres`, because some schema changes (for example CHECK-constraint edits) differ between the two engines. When you add a migration, add it to both directories.

### 📁 Project Structure

```
openrsvp/
├── cmd/openrsvp/main.go          # Entry point
├── internal/
│   ├── config/                    # Environment-based configuration
│   ├── database/                  # DB interface, SQLite/Postgres drivers, migrations
│   ├── auth/                      # Magic link authentication + middleware
│   ├── event/                     # Event CRUD
│   ├── rsvp/                      # RSVP system
│   ├── invite/                    # Invite card templates + customization
│   ├── message/                   # Organizer-attendee messaging
│   ├── comment/                   # Event page guestbook/comments
│   ├── webhook/                   # Webhook endpoints + SSRF-safe dispatcher
│   ├── notification/              # Email/SMS provider interface + implementations
│   ├── scheduler/                 # Background jobs (reminders, cleanup)
│   ├── security/                  # Rate limiting, honeypot, CSRF, sanitization
│   ├── stats/                     # Instance admin statistics (aggregate-only)
│   └── server/                    # HTTP server, router, embedded frontend
├── web/                           # SvelteKit frontend (Tailwind CSS)
├── Dockerfile                     # Multi-stage build
├── docker-compose.yml             # SQLite mode
└── docker-compose.postgres.yml    # PostgreSQL override
```

## ⚙️ Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for all options.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `ENV` | `development` | Environment (`development` or `production`) |
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite` or `postgres`) |
| `DB_DSN` | `/data/openrsvp.db` | Database connection string |
| `UPLOADS_DIR` | `/data/uploads` | Directory for uploaded files |
| `BASE_URL` | `http://localhost:8080` | Public URL for magic links and invites |
| `NOTIFICATION_EMAIL_PROVIDER` | `smtp` | Email provider (`smtp`, `sendgrid`, `ses`) |
| `DEFAULT_RETENTION_DAYS` | `30` | Days after event to auto-delete data |
| `FEEDBACK_GITHUB_TOKEN` | _(empty)_ | GitHub PAT for posting feedback as Issues |
| `FEEDBACK_GITHUB_REPO` | _(empty)_ | Target repo for Issues, e.g. `owner/repo` |
| `FEEDBACK_EMAIL` | _(empty)_ | Email address to receive feedback (fallback) |
| `TRUSTED_PROXIES` | _(empty)_ | Comma-separated CIDR ranges of trusted reverse proxies (e.g. `10.0.0.0/8,172.16.0.0/12`). When set, `X-Forwarded-For` / `X-Real-IP` headers are trusted to determine client IP. When empty (default), only `RemoteAddr` is used, which prevents IP spoofing. **Set this when running behind a reverse proxy (Nginx, Caddy, etc.)** |
| `MAX_COHOSTS_PER_EVENT` | `10` | Maximum number of co-hosts allowed per event |
| `ADMIN_EMAILS` | _(empty)_ | Comma-separated list of instance admin emails (e.g. `admin@example.com,ops@example.com`). Admin status is synced on every page load — add or remove emails and changes take effect immediately without requiring re-login |

### 📧 Email Providers

**SMTP** (default):

| Variable | Description |
|----------|-------------|
| `SMTP_HOST` | SMTP server hostname |
| `SMTP_PORT` | SMTP server port (default: `587`) |
| `SMTP_USERNAME` | SMTP username |
| `SMTP_PASSWORD` | SMTP password |
| `SMTP_FROM` | Sender email address |

**SendGrid** (`NOTIFICATION_EMAIL_PROVIDER=sendgrid`):

| Variable | Description |
|----------|-------------|
| `SENDGRID_API_KEY` | SendGrid API key (`SG.xxxxx`) |
| `SENDGRID_FROM` | Sender email address |

**AWS SES** (`NOTIFICATION_EMAIL_PROVIDER=ses`):

| Variable | Description |
|----------|-------------|
| `SES_REGION` | AWS region (e.g. `us-east-1`) |
| `SES_USERNAME` | SES SMTP username |
| `SES_PASSWORD` | SES SMTP password |
| `SES_FROM` | Sender email address |

### 📱 SMS Providers (Optional)

Set `NOTIFICATION_SMS_PROVIDER` to enable SMS notifications for reminders.

**Twilio** (`NOTIFICATION_SMS_PROVIDER=twilio`):

| Variable | Description |
|----------|-------------|
| `TWILIO_ACCOUNT_SID` | Twilio Account SID (`ACxxxxx`) |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token |
| `TWILIO_FROM_NUMBER` | Twilio sender phone number (`+15551234567`) |

**Vonage** (`NOTIFICATION_SMS_PROVIDER=vonage`):

| Variable | Description |
|----------|-------------|
| `VONAGE_API_KEY` | Vonage API key |
| `VONAGE_API_SECRET` | Vonage API secret |
| `VONAGE_FROM` | Sender name or number |

**Amazon SNS** (`NOTIFICATION_SMS_PROVIDER=sns`):

| Variable | Description |
|----------|-------------|
| `SNS_SMS_REGION` | AWS region (e.g. `us-east-1`) |
| `SNS_SMS_ACCESS_KEY_ID` | AWS access key ID |
| `SNS_SMS_SECRET_ACCESS_KEY` | AWS secret access key |

## 📡 API

All API endpoints are under `/api/v1`. The server also provides:

- `GET /health` — Health check
- `GET /health/ready` — Readiness check (includes DB connectivity)

### 🔑 Authentication

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/magic-link` | Request magic link |
| POST | `/api/v1/auth/verify` | Verify magic link token |
| POST | `/api/v1/auth/logout` | Logout |
| GET | `/api/v1/auth/me` | Get current user |
| GET | `/api/v1/auth/me/export` | Export all your data |
| DELETE | `/api/v1/auth/me` | Delete account and all associated data |

### 📅 Events

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/events` | Create event |
| GET | `/api/v1/events` | List your events |
| GET | `/api/v1/events/:id` | Get event |
| PUT | `/api/v1/events/:id` | Update event |
| POST | `/api/v1/events/:id/publish` | Publish event |
| POST | `/api/v1/events/:id/cancel` | Cancel event |
| POST | `/api/v1/events/:id/reopen` | Re-open cancelled event as draft |
| POST | `/api/v1/events/:id/duplicate` | Duplicate event |
| DELETE | `/api/v1/events/:id` | Delete event |

### 📋 RSVPs

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/rsvp/public/:shareToken` | Submit RSVP (public) |
| GET | `/api/v1/rsvp/public/token/:rsvpToken` | Get RSVP (public) |
| PUT | `/api/v1/rsvp/public/token/:rsvpToken` | Update RSVP (public) |
| GET | `/api/v1/rsvp/event/:eventId` | List RSVPs |
| GET | `/api/v1/rsvp/event/:eventId/stats` | RSVP stats |
| DELETE | `/api/v1/rsvp/event/:eventId/:attendeeId` | Remove attendee |

### 🎨 Invite Cards

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/invite/templates` | List templates |
| GET | `/api/v1/invite/event/:eventId` | Get invite card |
| PUT | `/api/v1/invite/event/:eventId` | Save invite card |
| GET | `/api/v1/invite/event/:eventId/preview` | Preview invite |

### 💬 Messages

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/messages/event/:eventId` | Send message (organizer) |
| GET | `/api/v1/messages/event/:eventId` | List messages (organizer) |
| POST | `/api/v1/messages/attendee/:rsvpToken` | Send message (attendee) |
| GET | `/api/v1/messages/attendee/:rsvpToken` | List messages (attendee) |

### ⏰ Reminders

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/reminders/event/:eventId` | Create reminder |
| GET | `/api/v1/reminders/event/:eventId` | List reminders |
| PUT | `/api/v1/reminders/:reminderId` | Update reminder |
| DELETE | `/api/v1/reminders/:reminderId` | Cancel reminder |

### 📝 Comments / Guestbook

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/comments/public/:shareToken` | List public comments (paginated) |
| POST | `/api/v1/comments/public/:shareToken` | Post comment (requires `X-RSVP-Token` header) |
| DELETE | `/api/v1/comments/public/:commentId` | Delete own comment (requires `X-RSVP-Token`) |
| GET | `/api/v1/comments/event/:eventId` | List all comments (organizer) |
| DELETE | `/api/v1/comments/event/:eventId/:commentId` | Delete any comment (organizer) |

### 🔗 Webhooks

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/webhooks/event/:eventId` | Create webhook |
| GET | `/api/v1/webhooks/event/:eventId` | List webhooks |
| GET | `/api/v1/webhooks/:webhookId` | Get webhook |
| PUT | `/api/v1/webhooks/:webhookId` | Update webhook |
| DELETE | `/api/v1/webhooks/:webhookId` | Delete webhook |
| POST | `/api/v1/webhooks/:webhookId/rotate-secret` | Rotate signing secret |
| GET | `/api/v1/webhooks/:webhookId/deliveries` | Delivery history |
| POST | `/api/v1/webhooks/:webhookId/test` | Send test webhook |

### 📥 CSV Import

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/rsvp/import/template` | Download CSV template |
| POST | `/api/v1/rsvp/event/:eventId/import/preview` | Preview CSV upload |
| POST | `/api/v1/rsvp/event/:eventId/import` | Execute confirmed import |

### 🔑 Instance Admin

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/admin/stats` | Instance-wide aggregate statistics (admin only) |

### 📊 Email Tracking

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/notifications/track/open/:logId` | Tracking pixel (public) |
| GET | `/api/v1/notifications/event/:eventId/stats` | Email delivery stats (organizer) |
| GET | `/api/v1/notifications/event/:eventId` | Delivery log (organizer) |
| POST | `/api/v1/notifications/webhooks/sendgrid` | Inbound SendGrid delivery events (public) |
| POST | `/api/v1/notifications/webhooks/ses` | Inbound SES delivery events (public) |

Open tracking is gated by `EMAIL_OPEN_TRACKING_ENABLED`. The inbound delivery webhooks record bounces and complaints; provider signature verification is a tracked follow-up (see [Known limitations](#known-limitations)).

### ✉️ Unsubscribe

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/unsubscribe?token=…` | Resolve an unsubscribe token (public) |
| POST | `/api/v1/unsubscribe` | Confirm unsubscribe — adds the address to the suppression list (public) |

### 🧭 Setup

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/setup/status` | Whether the instance has been configured (public) |
| GET | `/api/v1/setup/config` | Read non-secret instance settings (admin) |
| POST | `/api/v1/setup/config` | Update non-secret instance settings (admin) |

Non-secret instance settings (instance name, default timezone, allow-signups, support email) are stored in the database and overlaid on the environment config at startup. Secrets remain environment-only.

## 🏠 Self-Hosting Guide

### 🐳 Docker (recommended)

The fastest way to get a production instance running:

```yaml
# docker-compose.yml
services:
  openrsvp:
    image: ghcr.io/yannkr/openrsvp:latest
    restart: unless-stopped
    expose:
      - 8080
    environment:
      ENV: production
      BASE_URL: https://rsvp.yourdomain.com
      DB_DSN: /data/openrsvp.db
      UPLOADS_DIR: /data/uploads
      SMTP_HOST: smtp.yourdomain.com
      SMTP_PORT: 587
      SMTP_USERNAME: noreply@yourdomain.com
      SMTP_PASSWORD: yourpassword
      SMTP_FROM: noreply@yourdomain.com
    volumes:
      - ./data:/data
```

```bash
docker compose up -d
```

**Required variables for production:**

| Variable | Why it's required |
|----------|-------------------|
| `ENV=production` | Switches to JSON structured logging |
| `BASE_URL` | Used in magic links and invite emails — must be the public HTTPS URL |
| `SMTP_*` | Email delivery is required for magic link login |

> **Data persistence:** all state lives under `/data` (SQLite DB + uploads). Mount a volume there — losing it means losing all events and RSVPs.

### Reverse Proxy (Nginx)

```nginx
server {
    listen 443 ssl;
    server_name rsvp.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 💬 Feedback

The in-app feedback button requires at least one delivery channel. **If neither is configured, submissions return 200 OK but are silently discarded** — a warning is logged at startup.

**Option 1 — GitHub Issues (recommended)**

Create a [Personal Access Token](https://github.com/settings/tokens) with `repo` scope (classic) or Issues write permission (fine-grained):

```
FEEDBACK_GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
FEEDBACK_GITHUB_REPO=yourorg/yourrepo
```

Each submission opens an Issue titled `[Feedback - bug] …` with labels `feedback` and the feedback type.

**Option 2 — Email fallback**

Requires `SMTP_*` (or another email provider) to be configured:

```
FEEDBACK_EMAIL=feedback@yourdomain.com
```

GitHub Issues takes priority if both are set. Email is used as fallback when only `FEEDBACK_EMAIL` is provided.

### 💾 Backups

For SQLite, back up the database file:
```bash
sqlite3 /data/openrsvp.db ".backup /backups/openrsvp-$(date +%Y%m%d).db"
```

For PostgreSQL, use `pg_dump`:
```bash
docker compose exec postgres pg_dump -U openrsvp openrsvp > backup.sql
```

### ⚠️ Known limitations

**Inbound delivery webhooks are unauthenticated.** The SendGrid/SES delivery webhooks record bounces and complaints but do not yet verify provider signatures. Restrict access at the reverse proxy if you expose them. Signature verification is a tracked follow-up.

### 🔑 Operator note: rotate pre-v1.5.1 Postgres credentials

If you deployed `docker-compose.postgres.yml` **before v1.5.1**, rotate your Postgres password and confirm the Postgres port binds to `127.0.0.1` only. The old file shipped default `openrsvp:openrsvp` credentials and exposed the port on all interfaces; both were fixed in v1.5.1 (credentials now come from `.env`, port bound to localhost).

## 🧰 Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go with chi router |
| Frontend | SvelteKit + Tailwind CSS |
| Database | SQLite or PostgreSQL (both supported, both CI-tested) |
| Auth | Magic links (passwordless) |
| Notifications | SMTP, SendGrid, SES, Twilio, Vonage, SNS |
| Deployment | Docker (multi-stage, single binary) |

## 📝 Changelog

### v1.9.4 (2026-08-19)

**Fixes:**
- **Guest list pills hid plus-ones, so someone bringing extras looked like a party of one.** The invite and RSVP pages already counted plus-ones in the headcount, but each name chip only showed the person who RSVPed. Pills now include +n when that guest indicated they are bringing additional people

### v1.9.3 (2026-08-18)

**Fixes:**
- **Shared invite links showed a generic site title in chat previews.** iMessage, Slack, WhatsApp and similar crawlers never run JavaScript, so they always read "OpenRSVP — Beautiful Invitations, Zero Ads" from the SPA shell. Invite pages now serve `OpenRSVP - {event title}` (and matching Open Graph / Twitter title tags) so the preview names the event. Drafts and unknown tokens keep the generic title

### v1.9.2 (2026-08-17)

**Fixes:**
- **Invite card text was hard to read over a background photo.** Body copy sat at regular weight on top of the image, so date, location, and the message disappeared into busy photos. The card now uses medium weight by default and semibold when a photo is set; the wash over the image is a bit stronger, and the italic footer uses the same ink as the rest of the card

### v1.9.1 (2026-08-16)

**Features:**
- **Guests can now read the event description on the invite and RSVP pages.** Organizers already wrote one, and the public API already returned it, but neither guest page rendered it — so parking notes, gate codes and "bring a side" lived only on the organizer's event page. Both pages now show an Event Details card: the first few lines, then Show more / Show less for the rest. Short descriptions skip the fade and the toggle, so they are not truncated with no way to expand

### v1.9.0 (2026-08-16)

**Features:**
- **The dietary notes question is now optional per event.** It was rendered on every RSVP form, so events with no catering — a talk, a park meetup — still asked every guest for allergy information nobody needed. An "Ask for dietary notes" checkbox in the create wizard and the edit form now controls it, defaulting to on so existing events and organizers who ignore it see no change. With it off the question is hidden on the public invite page, the guest self-service page and the organizer guest list, and a `dietaryNotes` value posted by a client is ignored rather than rejected. Notes already collected are preserved, so turning the setting off and back on loses nothing, and the CSV export column stays unconditional to keep the export schema stable

**Tests & CI:**
- **The Playwright end-to-end suite only ran on one contributor's machine.** It extracted magic link tokens by shelling out to `docker compose logs` with the working directory pinned to an absolute path outside the repository, and the base URL was hardcoded across four files. Token retrieval is now strategy-based — `E2E_SERVER_LOG` reads the server's log file directly, otherwise it still falls back to Docker, but from a repository root located by walking up to `go.mod` — and the base URL comes from `E2E_BASE_URL`, shared from `helpers.ts` so the specs and the Playwright config cannot drift apart
- `make e2e` builds the binary, starts it on a scratch SQLite database, runs the suite and tears it down again, with no Docker required. It refuses to start when the port is already served, so a stale process cannot silently make the suite test an old build
- The end-to-end suite now runs in CI as a blocking gate. When it fails, the server log and Playwright traces are uploaded as artifacts, so the suite cannot quietly drift out of working order again

### v1.8.3 (2026-08-14)

**Fixes:**
- **Guest pages no longer render light and dark mode at the same time.** The invite, RSVP and unsubscribe pages painted a hardcoded light gradient behind cards and text that follow the theme tokens, so a guest in dark mode got a light page carrying black cards. The invite card was worse: its paper is a translucent tint of the organizer's colors, but its text used the app's neutral scale, which inverts — the event title rendered near-white on light paper and was effectively invisible. The page wash now comes from a themed `--page-gradient`, and the invite card is theme-independent: it carries its own ink palette over an opaque paper base, so it renders identically in the app, on the guest page and in email. Chalkboard and Clean Minimal, both unreadable in dark mode, are fixed by the same change
- **Invite cards rendered in the browser's default serif instead of the chosen font.** Font choices were stored and emitted as bare family names with no fallback, and the default was `Inter`, which the app does not self-host — so an unmatched family with no fallback fell through to Times. Choices now map to complete stacks, the picker offers the three self-hosted families (Satoshi, Plus Jakarta Sans, Geist Mono) alongside the web-safe ones, and the stored default is `Plus Jakarta Sans`. Existing `Inter` rows map onto it rather than showing a blank selection in the designer
- **Plus Jakarta Sans is a variable font that was being downloaded six times.** It was declared once per weight (300–800) against six byte-identical copies of the same file, which both wasted ~135 KB of transfer and made the declared weights unreliable. It is now declared once per style across the 200–800 range, and the body font is preloaded
- Text on solid accent fills (primary buttons, toasts) was white against the brighter dark-mode accents, around 2:1 contrast. A new `--color-on-accent` token flips it to near-black in dark mode
- `color-scheme` and `<meta name="theme-color">` now follow the active theme, and the pre-hydration background is painted from the resolved theme — so dark mode no longer flashes light on load or shows light browser chrome on mobile. Theme switching is centralized in `$lib/utils/theme` so the navbar and design-gallery toggles keep the attribute, the stored preference and the meta tag in sync

### v1.8.2 (2026-08-13)

**Fixes:**
- **Magic link verification no longer hangs on the spinner.** `/auth/verify` stripped the token from the URL with `replaceState` from `$app/navigation` *before* the try/catch. Users always arrive there on a fresh document load from their email client, where SvelteKit's router is not yet initialized, so the call threw — aborting the page before `POST /api/v1/auth/verify` was ever sent, and, being outside the catch, never setting the error state. The result was an endless "Verifying your login" spinner and no way to sign in. The call now falls back to the native `history.replaceState` when the router is not ready, so the token is still stripped and verification always proceeds

### v1.8.1 (2026-07-26)

**Fixes:**
- Event-series endpoints classified errors the opposite way round to the v1.8.0 bug: `POST /series` reported *every* failure as `400 bad_request`, and the update/stop handlers fell through to the same branch. A database outage was therefore blamed on the caller — and since the handler echoed `err.Error()` verbatim, the raw driver text (`create event series: sql: database is closed`) was returned to the client. Genuine failures now log server-side and return a generic `500 internal_error`; validation still returns 400 with its message, via the same `errcode.ErrValidation` sentinel as the rest of the API

### v1.8.0 (2026-07-26)

**Fixes:**
- **Validation errors no longer return HTTP 500.** Handlers classified errors with hardcoded allowlists of message *prefixes*, so any message not on the list fell through to `500 internal_error` with an unactionable `ERR-XXXXXXXX` reference. In production this blocked real guests from RSVPing: a phone number the E.164 check rejected produced "an internal error occurred" instead of "invalid phone format" (67 occurrences across 18 distinct clients; one event saw 4 submission attempts and 0 successes). Classification now uses an `errcode.ErrValidation` sentinel matched with `errors.Is`, so new validation messages are covered automatically instead of failing open to 500
- The same defect is fixed in the event, comment, and CSV-import handlers. Event creation was the worst affected: the allowlist read `event_date is required` while the service raises `eventDate is required`, so that error — plus over-long title/description/location, invalid `contactRequirement`, `maxCapacity` bounds, an RSVP deadline after the event date, and unparseable dates — all reached organizers as 500s
- Phone numbers are normalized before validation, so `+32 479 12 34 56`, `+1 (415) 555-2671` and `+33.6.12.34.56.78` are accepted and stored as bare E.164. A national number with no country code is still rejected, now with an actionable message
- The invite form's phone placeholder was `+1 (555) 123-4567`, itself invalid under the server's E.164 check — the UI demonstrated a format the API rejects. Replaced with a valid example plus a country-code hint, and the field is normalized client-side

**Tests:**
- Deadline-enforcement fixtures used a hardcoded "future" date that had since passed, failing the suite regardless of code changes; they are now relative to `time.Now()`

### v1.7.0 (2026-06-10)

**Database:**
- PostgreSQL is now functional. A `?` → `$N` placeholder rewriter covers the stores and the transactional (`*sql.Tx`) paths, per-column boolean handling works with `lib/pq`, and a notification timestamp scan fix lands the last edge case
- Migrations split into per-dialect directories (`internal/database/migrations/sqlite`, `internal/database/migrations/postgres`) so CHECK-constraint changes use `ALTER CONSTRAINT` on Postgres
- CI runs the entire test suite against `postgres:16` alongside SQLite

**CI:**
- Re-gate `golangci-lint` and `govulncheck` as blocking. golangci-lint runs on the go1.26-built action (v2.12.2) without panicking, and govulncheck on go1.26.4 clears GO-2026-5037/5039
- Burned down ~176 pre-existing lint findings so the gate is genuine

### v1.6.0 (2026-06-10)

**Security:**
- Pin all GitHub Actions to commit SHAs; add least-privilege `permissions: contents: read` to `ci.yml`
- CSRF: authenticated requests now require a session-bound (HMAC) token, the token cookie is rebound at/after login, and the `/api/v1/auth/` CSRF exclusion is narrowed to the pre-auth magic-link/verify endpoints only, so `PATCH /auth/me` and `POST /auth/logout` are now CSRF-protected
- Webhooks: delivery URLs must be `https://` in production (`http://` allowed only in development)
- Web app: the session token no longer lives in `localStorage`; the SPA authenticates via the existing HttpOnly `session` cookie (Bearer is still accepted server-side for API/CLI clients). The magic-link token is stripped from browser history after use
- Repo hygiene: tooling directories moved into a tracked `.gitignore`, vendored e2e `node_modules` untracked, `.dockerignore` tightened

**Features:**
- Email delivery tracking is now functional. An open-tracking pixel is embedded in HTML emails (gated by `EMAIL_OPEN_TRACKING_ENABLED`), inbound SendGrid/SES delivery webhooks (`POST /api/v1/notifications/webhooks/sendgrid|ses`) record bounces and complaints, and the stats dashboard shows real numbers instead of always-zero. Provider webhook signature verification is a documented follow-up
- Email unsubscribe and suppression list. Reminder and message emails carry an unsubscribe footer, a public token-based unsubscribe page lives at `/unsubscribe`, and suppressed addresses are skipped before sending. Migration 000030
- Account deletion and data export. Organizers can export all their data (`GET /api/v1/auth/me/export`) and permanently delete their account and all associated data (`DELETE /api/v1/auth/me`) from a new Account settings page (`/account`)
- Setup wizard. DB-backed non-secret instance settings (instance name, default timezone, allow-signups, support email) via `/setup`, stored in `instance_config` (migration 000031) and overlaid on the environment config at startup. Secrets remain environment-only
- Guest comment deletion. Guests can delete their own guestbook comments
- Guest feedback. A "Report a problem" widget on public RSVP pages lets guests submit feedback without logging in

**Tests & CI:**
- Large test-coverage increase (overall ~51% to ~61%): webhook dispatcher/HMAC/SSRF, the reminder pipeline plus its double-send lock, all email/SMS providers including SMTP header-injection defense, and an end-to-end server `httptest` suite
- CI gained coverage reporting, a `govulncheck` job, a `golangci-lint` job (plus `.golangci.yml`), and a Go 1.24/1.25 matrix

**Known limitations:**
- PostgreSQL support is experimental and not functional in this release. The stores use `?` SQL placeholders with no rewrite layer (and transactional paths use raw `*sql.Tx`), while `lib/pq` requires `$1, $2, …`, so queries fail under Postgres. SQLite is the supported and tested database; a Postgres compatibility fix is tracked as a follow-up

### v1.5.2 (2026-05-29)

**Dependency upgrades + CVE patches:**
- Bump `golang.org/x/net` v0.40.0 → v0.55.0 (fixes GO-2026-4918: net/http2 infinite loop on bad `SETTINGS_MAX_FRAME_SIZE`) and `golang.org/x/sys` v0.33.0 → v0.45.0
- Add explicit `toolchain go1.26.3` directive to `go.mod` so local builds and CI (not just Docker) compile against the patched std-lib that fixes GO-2026-4980/4982 (html/template XSS escaper bypass), GO-2026-4977/4986 (net/mail quadratic concat), and GO-2026-4971 (net Dial NUL byte). `go` directive bumped 1.23.0 → 1.25.0 (required by upgraded modules)
- Upgrade all remaining Go modules to latest patch/minor: `aws-sdk-go-v2` suite, `go-chi/cors` v1.2.1 → v1.2.2, `golang-migrate/migrate/v4` v4.18.2 → v4.19.1, `lib/pq` v1.10.9 → v1.12.3, `mattn/go-sqlite3` v1.14.24 → v1.14.44, `rs/zerolog` v1.33.0 → v1.35.1, `smithy-go` v1.24.1 → v1.26.0
- Backend `govulncheck` clean: **0 vulnerabilities** (was 6 callable std-lib/x-net vulns)
- Frontend majors: `vite` 7 → 8, `typescript` 5 → 6, `@sveltejs/vite-plugin-svelte` 6 → 7; minors: `@sveltejs/kit` → 2.61.1, `svelte` → 5.55.10, `tailwindcss`/`@tailwindcss/vite` → 4.3.0, `@playwright/test` → 1.60.0, `svelte-check` → 4.4.8
- Add npm `overrides` pinning `cookie` ^0.7.2 (fixes GHSA-pxg6-pf52-xh8x: out-of-bounds chars in cookie name/path/domain; SvelteKit still transitively pins `cookie@^0.6.0`). Frontend `npm audit` clean: **0 vulnerabilities**

**Security hardening:**
- Eliminate modulo bias in base62 token generation for RSVP guest tokens (`internal/rsvp/service.go`) and event share tokens (`internal/event/service.go`) — switch to rejection sampling so all 62 alphabet characters are uniformly distributed (previously the first 8 characters were ~25% more likely)

### v1.5.1

**CVE patches:**
- Bump `github.com/go-chi/chi/v5` v5.2.1 → v5.3.0 (fixes GO-2025-3770: Host header injection → open redirect in `RedirectSlashes`)
- Bump `golang.org/x/net` v0.33.0 → v0.40.0 (fixes GO-2025-3595: html.Tokenizer incorrect input neutralization called via bluemonday)
- Bump Dockerfile base image `golang:1.23-alpine` → `golang:1.26-alpine` to pick up patched std-lib (go1.26.3): html/template XSS escaper bypass (GO-2026-4980), net/mail quadratic concat (GO-2026-4977), net/http2 frame infinite loop (GO-2026-4918)
- Frontend: `npm audit fix` resolved 3 high / 2 moderate / 2 low CVEs in svelte, vite, @sveltejs/kit, devalue, picomatch
- Production `govulncheck` clean: **0 callable vulnerabilities** in the shipped binary
- Fix race condition in `TestExecuteCSVImport_SendInvitations` — switch shared slice to `atomic.Int32` so the asyncNotify goroutines cannot race with the test goroutine

**Security hardening (full audit pass):**
- Fix HTML injection in feedback email body — user-supplied `message` is now HTML-escaped before being embedded in the operator email (`internal/feedback/service.go`)
- Remove hardcoded Postgres credentials from `docker-compose.postgres.yml`; the file now requires `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` from `.env` and binds the Postgres port to `127.0.0.1` only
- Add global `SecurityHeadersMiddleware` — sets `X-Content-Type-Options`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Cross-Origin-Opener-Policy: same-origin`, and `Strict-Transport-Security` (HTTPS only) on every response
- Defang CSV formula injection on export — cells starting with `=`, `+`, `-`, `@`, tab, or CR are now prefixed with a single quote so opening the file in Excel/Sheets/Calc cannot execute attacker-supplied formulas (DDE, HYPERLINK exfil)
- Validate invite-card customization server-side — `primaryColor`/`secondaryColor` must match `#hex`, `font` is allowlisted, and `customData.backgroundImage` must be a `/`-relative path or http(s) URL with no CSS-breakout characters; client-side `InviteCardPreview` re-validates the URL as defense-in-depth
- Strengthen image-upload validation — every uploaded image is verified against its format-specific magic bytes (PNG signature, JPEG SOI+EOI markers, RIFF/WEBP), not just `http.DetectContentType`
- Align session cookie `MaxAge` with `cfg.SessionExpiry` so the browser cookie no longer outlives the server-side session
- Add `ReadHeaderTimeout` (5s) and `MaxHeaderBytes` (1MB) to the HTTP server to defeat slowloris
- Defensive CRLF stripping on SMTP `From` / `To` / `Subject` headers

### v1.5.0

**Design System Overhaul:**
- New warm rose (`#E54666`) brand palette replacing indigo, with stone (warm gray) neutrals
- Satoshi (display), Plus Jakarta Sans (body), and Geist Mono (data) fonts — self-hosted, no CDN dependency
- Semantic color tokens via Tailwind CSS v4 `@theme` — change your brand in one file (`app.css`)
- Full dark mode with toggle in navbar, localStorage persistence, and system preference detection
- `bg-surface` token for cards/modals that flips white→dark automatically
- All 17 shared UI components updated to semantic design tokens
- All 19 page files updated with consistent design language
- InviteCardPreview: all 10 invite themes reworked with design system defaults
- Go `EmailColors` struct — email template colors defined once, used across 9 templates
- `/design` route: live component gallery showcasing all variants in light + dark mode
- Playwright visual regression tests for automated screenshot comparison
- Email rendering test framework via mailpit integration
- `DESIGN.md` — documented color system, typography, spacing, motion, and shadows
- Toast system deduplicated (single `Toast.svelte` component)

### v1.4.2

**Features:**
- Add instance admin dashboard with aggregate statistics — total events, guests, organizers, RSVP distribution, notification health, and feature adoption metrics
- Add `ADMIN_EMAILS` env var for instance admin role — comma-separated list of admin email addresses, synced on every page load (no re-login required to grant or revoke)
- Add `RequireAdmin` middleware — admin endpoints return 403 for non-admin users
- Privacy by design: all statistics are aggregate-only (COUNT, AVG, SUM) — no individual user data or PII is ever returned

**Backend:**
- New `internal/stats/` package with model, store, service (5-minute in-memory cache), and handler
- New database migration (000029): adds `is_admin` column to organizers table
- `GET /api/v1/admin/stats` endpoint with auth + admin middleware
- Admin status synced from `ADMIN_EMAILS` on session validation (not just login)

**Frontend:**
- New `/admin` dashboard page with metric cards, bar charts, notification health grid, and feature adoption breakdown
- Conditional "Admin" link in navbar (visible only to admins)
- Admin layout with auth + admin guard (redirects non-admins to /events)

### v1.4.1

**Performance:**
- Prerender landing page — full HTML delivered on first byte instead of blank SPA shell (19KB prerendered vs 1.2KB empty)
- Remove auth-blocking spinner from public pages — landing, invite, and RSVP pages render instantly without waiting for `/auth/me`
- Move auth loading gate to `/events` layout only, where it belongs
- Add gzip compression middleware (level 5) for HTML, CSS, JS, JSON, and SVG responses
- Add `Cache-Control` headers — `immutable` for Vite-hashed assets, `no-cache` for HTML to ensure safe updates
- Add inline critical CSS and fallback meta tags to `app.html` for faster first paint and SEO
- Enable SSR for prerenderable routes; separate SPA fallback (`200.html`) from prerendered `index.html`

### v1.4.0

**Features:**
- Add event guestbook/comments — authenticated attendees can post comments on public event pages with cursor-based pagination, rate limiting (5/hour), bluemonday sanitization, and organizer moderation
- Add webhooks/API events — organizers can register webhook endpoints per event, with HMAC-SHA256 payload signing, SSRF-safe delivery (private IP blocking, no redirects), exponential backoff retries, delivery history, test endpoints, and secret rotation
- Add CSV guest list import — upload CSV files with flexible column aliases (e.g. "full name" → name), preview with validation, duplicate detection, and optional invitation sending. Includes downloadable CSV template
- Add email delivery tracking — tracking pixel for open detection, delivery status progression (unknown → delivered → opened → clicked), bounce/complaint handling, and per-event email statistics dashboard
- Add comments_enabled toggle on events (enabled by default)
- Add import_source field on attendees to track CSV-imported guests

**Frontend:**
- Add guestbook section on public invite page with comment posting and pagination
- Add comments section on organizer event detail page with moderation (delete)
- Add CSV Import page with drag-and-drop upload, preview table, and step-by-step wizard
- Add Webhook management page with create/edit/delete, delivery history, test webhook, and secret rotation
- Add email delivery stats section on organizer event detail page

**Backend:**
- 5 new database migrations (000024–000028): notification tracking columns, event_comments table, comments_enabled column, webhooks + webhook_deliveries tables, attendee import_source column
- Webhook dispatch integrated into RSVP created, event published, and event cancelled callbacks
- Notification service extended with SendResult.MessageID capture for delivery tracking correlation

### v1.3.1

**Fix:**
- Fix timezone handling: event times now use the selected event timezone instead of the browser's local timezone for creation, editing, and display. Previously, entering 11:11 AM for a UTC-7 event from a UTC browser would store/display as 4:11 AM. Added `datetimeLocalToUTC` and `utcToDatetimeLocal` utilities; updated all event date formatting to pass the event timezone to `Intl.DateTimeFormat`.
- Fix Go module path and all GitHub/GHCR references to use `github.com/yannkr/openrsvp`

**CI:**
- Skip CI workflow on tag pushes to avoid double Docker build with the Release workflow

### v1.3.0

**Features:**
- Add event series with recurring event support (daily, weekly, monthly frequencies)
- Add co-host management — invite other organizers to manage your event
- Add waitlist with automatic promotion when spots open up
- Add custom RSVP questions (text, select, checkbox types) with drag-and-drop reordering
- Add co-host email notification when added to an event
- Add event date to organizer RSVP notification email subject for recurring event disambiguation

**Security:**
- Add `X-Content-Type-Options: nosniff`, `Content-Security-Policy`, and `X-Frame-Options` headers on uploaded file serving
- Add email and phone format validation via `security.ValidateEmail` / `security.ValidatePhone`
- Add field length limits: name (200), email (254), phone (20), dietary notes (500), event title (200), description (5,000), location (500), message subject (200), message body (10,000)
- Add message rate limiting: 1 per minute for organizers, 1 per 5 minutes for attendees
- Fix RSVP concurrency: per-event mutex on `RemoveAttendee` and `UpdateAttendeeAsOrganizer` to prevent capacity over-subscription
- Add notification semaphore (cap 100) to bound concurrent notification goroutines
- Add error reference codes (ERR-XXXXXXXX) — 500 responses no longer leak internal error details; codes correlate with server logs
- Add co-host notification throttle (1 per hour per event:email pair) to prevent spam
- Add per-event mutex on co-host add to prevent TOCTOU race on count check
- Add 200ms timing floor on co-host add endpoint to prevent email enumeration via timing side channel
- Add per-IP rate limiter (10/min) on co-host add endpoint
- Make co-host limit configurable via `MAX_COHOSTS_PER_EVENT` env var (default 10)

### v1.2

- Security: `middleware.RealIP` is now conditional on `TRUSTED_PROXIES` — prevents clients from spoofing their IP via `X-Forwarded-For` to bypass rate limiting
- Security: CSRF tokens are now bound to the session cookie via HMAC-SHA256 — a token issued for one session cannot be replayed against another
- Security: CSRF cookie is no longer regenerated on every GET request (only set when absent)
- Security: RSVP lookup now sends a magic link email instead of returning the token directly (prevents email enumeration)
- Fix: dashboard stats (attending, maybe, declined, headcount) now refresh after editing or removing attendees
- Fix: max attendees validation rejects non-numeric input on both create and edit forms
- Fix: rate limiting scoped to API routes only (no longer affects static SPA assets)
- Add: rate limit handling (429) in frontend API client

### v1.1.1

- Add calendar integration (.ics download and Add to Calendar button)
- Add CSV export for guest lists with status filtering
- Add RSVP deadlines with countdown display on invite page
- Add capacity limits with real-time spots-remaining display
- Add feedback follow-up opt-in checkbox and confirmation email
- Show headcount and guest list visibility toggles for events
- Default contact requirement to email-only when creating events
- Use shared email template for magic link sign-in email
- Add production Docker setup guide to self-hosting docs
- Warn at startup when no feedback channel is configured

### v1.1.0

- Default DB_DSN and UPLOADS_DIR to `/data` instead of relative paths

### v1.0.1

- SMS enable/disable controlled by `NOTIFICATION_SMS_PROVIDER` env var; email always required when SMS is off
- Public config endpoint (`GET /api/v1/config`) exposes feature flags to frontend
- Backend rejects phone-only contact requirement and sms contact method when SMS is disabled
- Frontend hides "Phone only" option and enforces email-required on RSVP forms when SMS is off
- Add Amazon SNS as an SMS notification provider
- Fix CORS to restrict allowed origins to configured BASE_URL
- Add request body size limit (1MB) to prevent DoS via large payloads
- Fix path traversal vulnerability in uploads endpoint
- Add event ownership checks on RSVP, message, reminder, and invite endpoints
- Sanitize internal error messages in HTTP responses
- Fix reminder CHECK constraint to allow 'processing' status
- Add unique indexes on attendees(event_id, email) and (event_id, phone)
- Fix warnExpiring to preserve event 'published' status for active RSVPs
- Add panic recovery in scheduler jobs and notification goroutines
- Fix rate limiter to key on IP address (strip port) with periodic cleanup
- Add notification send retry with exponential backoff
- Add CSRF token handling in frontend API client
- Wrap VerifyMagicLink in a database transaction
- Add PostgreSQL connection pool lifetime settings
- Add timeout to GitHub feedback API client
- Validate ENV, PORT, and DEFAULT_RETENTION_DAYS in config loading
- Return error from SNS provider constructor on AWS config failure

### v1.0.0 (2026-02-28)

- Event management: cancel confirmation modal, re-open cancelled events as draft, duplicate events (copies invite card design)
- Confirmation modals for removing attendees and cancelling reminders
- Inline editing for attendees (name, email, phone, status, dietary notes, plus-ones) and reminders
- Configurable RSVP contact requirements (email, phone, both, or either)
- Organizer email notifications on new RSVPs
- Two-way organizer-attendee messaging with email delivery
- Scheduled reminders with automatic defaults on publish (1 week + 3 days before)
- Feedback system with GitHub Issues integration and email fallback
- RSVP confirmation emails to attendees
- Pluggable notification providers: SMTP, SendGrid, SES (email); Twilio, Vonage (SMS)
- Security middleware: rate limiting, honeypot bot protection, CSRF tokens, HTML sanitization
- Data retention policy with warning emails and automatic cleanup
- Invite card designer with 5 templates, custom colors/fonts, background image uploads
- Magic link passwordless authentication
- SQLite (default) and PostgreSQL support
- Single-container Docker deployment with health checks
- Docker one-liner quick start

## 🤝 Contributing

Contributions are welcome! Here's how to get started:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

[MIT](LICENSE)
