# OpenRSVP

A self-hosted, privacy-first alternative to Evite. Create beautiful event invitations, manage RSVPs, and communicate with guests — all without ads or data tracking. Perfect for birthday parties, gatherings, and celebrations.

## Features

- **Beautiful Invitation Templates** — 5 customizable themes (Balloon Party, Confetti, Unicorn Magic, Superhero, Garden Picnic) with custom colors, fonts, and text
- **Passwordless Auth** — Magic link sign-in, no passwords to manage
- **Easy RSVPs** — Guests respond with one click, no account needed. Track dietary needs and plus-ones
- **Notifications** — Pluggable email (SMTP, SendGrid, SES) and SMS (Twilio, Vonage) providers
- **Messaging** — Two-way communication between organizers and attendees
- **Scheduled Reminders** — Automatic event reminders to guests
- **Privacy by Design** — Data auto-deletes after a configurable retention period (default 30 days post-event)
- **Bot Protection** — Honeypot fields and IP-based rate limiting
- **Self-Hosted** — Single Docker container, you own your data
- **SQLite or PostgreSQL** — SQLite by default, PostgreSQL for larger deployments

## Quick Start

### Docker One-Liner

```bash
docker run -d -p 8080:8080 -v openrsvp-data:/data -e BASE_URL=http://localhost:8080 ghcr.io/openrsvp/openrsvp:latest
```

Visit http://localhost:8080

### Docker Compose

```bash
git clone https://github.com/openrsvp/openrsvp.git
cd openrsvp
cp .env.example .env
docker compose up -d
```

### With PostgreSQL

```bash
docker compose -f docker-compose.yml -f docker-compose.postgres.yml up -d
```

## Development

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

### Project Structure

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
│   ├── notification/              # Email/SMS provider interface + implementations
│   ├── scheduler/                 # Background jobs (reminders, cleanup)
│   ├── security/                  # Rate limiting, honeypot, CSRF, sanitization
│   └── server/                    # HTTP server, router, embedded frontend
├── web/                           # SvelteKit frontend (Tailwind CSS)
├── Dockerfile                     # Multi-stage build
├── docker-compose.yml             # SQLite mode
└── docker-compose.postgres.yml    # PostgreSQL override
```

## Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for all options.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `ENV` | `development` | Environment (`development` or `production`) |
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite` or `postgres`) |
| `DB_DSN` | `openrsvp.db` | Database connection string |
| `BASE_URL` | `http://localhost:8080` | Public URL for magic links and invites |
| `NOTIFICATION_EMAIL_PROVIDER` | `smtp` | Email provider (`smtp`, `sendgrid`, `ses`) |
| `DEFAULT_RETENTION_DAYS` | `30` | Days after event to auto-delete data |

## API

All API endpoints are under `/api/v1`. The server also provides:

- `GET /health` — Health check
- `GET /health/ready` — Readiness check (includes DB connectivity)

### Authentication

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/magic-link` | Request magic link |
| POST | `/api/v1/auth/verify` | Verify magic link token |
| POST | `/api/v1/auth/logout` | Logout |
| GET | `/api/v1/auth/me` | Get current user |

### Events

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

### RSVPs

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/rsvp/public/:shareToken` | Submit RSVP (public) |
| GET | `/api/v1/rsvp/public/token/:rsvpToken` | Get RSVP (public) |
| PUT | `/api/v1/rsvp/public/token/:rsvpToken` | Update RSVP (public) |
| GET | `/api/v1/rsvp/event/:eventId` | List RSVPs |
| GET | `/api/v1/rsvp/event/:eventId/stats` | RSVP stats |
| DELETE | `/api/v1/rsvp/event/:eventId/:attendeeId` | Remove attendee |

### Invite Cards

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/invite/templates` | List templates |
| GET | `/api/v1/invite/event/:eventId` | Get invite card |
| PUT | `/api/v1/invite/event/:eventId` | Save invite card |
| GET | `/api/v1/invite/event/:eventId/preview` | Preview invite |

### Messages

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/messages/event/:eventId` | Send message (organizer) |
| GET | `/api/v1/messages/event/:eventId` | List messages (organizer) |
| POST | `/api/v1/messages/attendee/:rsvpToken` | Send message (attendee) |
| GET | `/api/v1/messages/attendee/:rsvpToken` | List messages (attendee) |

### Reminders

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/reminders/event/:eventId` | Create reminder |
| GET | `/api/v1/reminders/event/:eventId` | List reminders |
| PUT | `/api/v1/reminders/:reminderId` | Update reminder |
| DELETE | `/api/v1/reminders/:reminderId` | Cancel reminder |

## Self-Hosting Guide

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

### Backups

For SQLite, back up the database file:
```bash
sqlite3 /data/openrsvp.db ".backup /backups/openrsvp-$(date +%Y%m%d).db"
```

For PostgreSQL, use `pg_dump`:
```bash
docker compose exec postgres pg_dump -U openrsvp openrsvp > backup.sql
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go with chi router |
| Frontend | SvelteKit + Tailwind CSS |
| Database | SQLite (default) / PostgreSQL |
| Auth | Magic links (passwordless) |
| Notifications | SMTP, SendGrid, SES, Twilio, Vonage |
| Deployment | Docker (multi-stage, single binary) |

## Changelog

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

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

[MIT](LICENSE)
