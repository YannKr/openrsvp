package security

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

var (
	// strictPolicy strips all HTML tags.
	strictPolicy *bluemonday.Policy
	// messagePolicy allows limited HTML suitable for message bodies.
	messagePolicy *bluemonday.Policy

	emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	phoneRegexp = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
)

func init() {
	strictPolicy = bluemonday.StrictPolicy()

	messagePolicy = bluemonday.NewPolicy()
	messagePolicy.AllowElements("b", "i", "em", "strong", "a", "br", "p", "ul", "ol", "li")
	messagePolicy.AllowAttrs("href").OnElements("a")
	messagePolicy.RequireNoFollowOnLinks(true)
}

// SanitizeStrict strips all HTML tags from the input string.
// The result is unescaped because bluemonday HTML-encodes characters like
// apostrophes (&#39;) which is wrong for plain-text storage.
func SanitizeStrict(input string) string {
	return html.UnescapeString(strictPolicy.Sanitize(input))
}

// SanitizeMessage allows limited HTML for message bodies. Permitted elements
// are: b, i, em, strong, a (with href), br, p, ul, ol, li.
func SanitizeMessage(input string) string {
	return messagePolicy.Sanitize(input)
}

// ValidateEmail validates that the given string is a well-formed email address.
func ValidateEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	return emailRegexp.MatchString(email)
}

// ValidatePhone validates that the given string is in E.164 phone format
// (e.g. +14155552671).
func ValidatePhone(phone string) bool {
	return phoneRegexp.MatchString(phone)
}

// SanitizeMiddleware returns middleware that intercepts JSON request bodies
// and sanitizes all string values using the strict policy before passing the
// request to the next handler.
func SanitizeMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contentType := r.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, "application/json") || r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Only process non-empty bodies that look like JSON.
			if len(body) == 0 {
				r.Body = io.NopCloser(bytes.NewReader(body))
				next.ServeHTTP(w, r)
				return
			}

			var data interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				// If the body isn't valid JSON, pass it through unchanged.
				r.Body = io.NopCloser(bytes.NewReader(body))
				next.ServeHTTP(w, r)
				return
			}

			sanitized := sanitizeValue(data)
			newBody, err := json.Marshal(sanitized)
			if err != nil {
				// Fall back to the original body on marshal failure.
				r.Body = io.NopCloser(bytes.NewReader(body))
				next.ServeHTTP(w, r)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(newBody))
			r.ContentLength = int64(len(newBody))
			next.ServeHTTP(w, r)
		})
	}
}

// sanitizeValue recursively walks through a decoded JSON structure and
// sanitizes all string values using the strict policy.
func sanitizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return SanitizeStrict(val)
	case map[string]interface{}:
		for k, v2 := range val {
			val[k] = sanitizeValue(v2)
		}
		return val
	case []interface{}:
		for i, v2 := range val {
			val[i] = sanitizeValue(v2)
		}
		return val
	default:
		return v
	}
}
