package auth

import (
	"context"
	"net/http"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const organizerContextKey contextKey = "organizer"

// RequireAuth returns middleware that validates the session token from the
// request cookie or Authorization header. If valid, it stores the organizer
// in the request context. If invalid, it responds with 401 Unauthorized.
func RequireAuth(service *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			organizer, err := service.ValidateSession(r.Context(), token)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			ctx := ContextWithOrganizer(r.Context(), organizer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ContextWithOrganizer returns a new context with the given organizer stored.
func ContextWithOrganizer(ctx context.Context, organizer *Organizer) context.Context {
	return context.WithValue(ctx, organizerContextKey, organizer)
}

// OrganizerFromContext extracts the organizer from the context.
// Returns nil if no organizer is stored.
func OrganizerFromContext(ctx context.Context) *Organizer {
	organizer, _ := ctx.Value(organizerContextKey).(*Organizer)
	return organizer
}
