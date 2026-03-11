# 🎉 OpenRSVP

A self-hosted, privacy-first alternative to Evite. Create beautiful event invitations, manage RSVPs, and communicate with guests — all without ads or data tracking. Perfect for birthday parties, gatherings, and celebrations.

## ✨ Features

- 🎨 **Beautiful Invitation Templates** — 5 customizable themes (Balloon Party, Confetti, Unicorn Magic, Superhero, Garden Picnic) with custom colors, fonts, and text
- 🔐 **Passwordless Auth** — Magic link sign-in, no passwords to manage
- 📋 **Easy RSVPs** — Guests respond with one click, no account needed. Track dietary needs and plus-ones
- 📬 **Notifications** — Pluggable email (SMTP, SendGrid, SES) and SMS (Twilio, Vonage, SNS) providers
- 💬 **Messaging** — Two-way communication between organizers and attendees
- ⏰ **Scheduled Reminders** — Automatic event reminders to guests
- 📝 **Guestbook** — Attendees can leave comments on event pages with organizer moderation
- 📥 **CSV Import** — Bulk import guest lists from CSV files with validation and duplicate detection
- 🔗 **Webhooks** — Real-time HTTP callbacks for RSVP and event lifecycle events with HMAC signing
- 📊 **Email Tracking** — Delivery status, open tracking, and per-event email statistics
- 🛡️ **Privacy by Design** — Data auto-deletes after a configurable retention period (default 30 days post-event)
- 🤖 **Bot Protection** — Honeypot fields and IP-based rate limiting
- 📈 **Instance Admin** — Aggregate dashboard for instance-wide statistics (events, guests, RSVP rates, notification health, feature adoption) — privacy-first, no individual tracking
- 🏠 **Self-Hosted** — Single Docker container, you own your data
- 🗄️ **SQLite or PostgreSQL** — SQLite by default, PostgreSQL for larger deployments

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

- Go 1.23+
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

## 🧰 Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go with chi router |
| Frontend | SvelteKit + Tailwind CSS |
| Database | SQLite (default) / PostgreSQL |
| Auth | Magic links (passwordless) |
| Notifications | SMTP, SendGrid, SES, Twilio, Vonage, SNS |
| Deployment | Docker (multi-stage, single binary) |

## 📝 Changelog

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
