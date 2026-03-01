# OpenRSVP — Claude Code Instructions

## Build & Test Commands

- `make build` — builds frontend then Go binary with embedded frontend
- `make dev` — runs Go backend only (use `cd web && npm run dev` for frontend)
- `CGO_ENABLED=1 go test ./... -race` — run full test suite
- `CGO_ENABLED=1 go test ./internal/event/... -v -race` — run a single package
- `cd web && npm run build` — build frontend only
- `go build ./...` — check Go compilation

## Code Style

- Go: standard library style, no linter config — keep it simple
- Frontend: Svelte 5 runes (`$state`, `$derived`), Tailwind CSS v4, TypeScript
- SQL: `?` placeholders (works with both SQLite and lib/pq)
- UUIDv7 for all IDs, RFC3339 for timestamps
- Each domain follows model/store/service/handler layers

## Changelog

When adding features, fixing bugs, or making notable changes, update the **Changelog** section in `README.md`. Add entries under the current version heading. If a new version is tagged, create a new heading for it.
