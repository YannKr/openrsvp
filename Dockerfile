# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.23-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Embed frontend build into Go binary
RUN rm -rf internal/server/frontend && \
    mkdir -p internal/server/frontend && \
    cp -r /app/web/build/* internal/server/frontend/ 2>/dev/null || true
COPY --from=frontend /app/web/build ./internal/server/frontend/
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /openrsvp ./cmd/openrsvp

# Stage 3: Final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S openrsvp && adduser -S openrsvp -G openrsvp
COPY --from=backend /openrsvp /usr/local/bin/openrsvp
RUN mkdir -p /data /data/uploads && chown -R openrsvp:openrsvp /data
USER openrsvp
ENV DB_DSN=/data/openrsvp.db
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
ENTRYPOINT ["openrsvp"]
