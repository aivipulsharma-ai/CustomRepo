package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/dextr_avs/price-feeder/config"
	"github.com/dextr_avs/price-feeder/models"
)

type OneInchAuthMiddleware struct {
	config *config.Config
}

func NewOneInchAuthMiddleware(cfg *config.Config) *OneInchAuthMiddleware {
	return &OneInchAuthMiddleware{
		config: cfg,
	}
}

func (m *OneInchAuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if credentials are configured
		if m.config.OneInchAuth.AccessKey == "" || m.config.OneInchAuth.SecretKey == "" {
			m.writeAuthError(w, "1inch authentication not configured")
			return
		}

		// Get required headers
		accessKey := r.Header.Get("INCH-ACCESS-KEY")
		timestamp := r.Header.Get("INCH-ACCESS-TIMESTAMP")
		signature := r.Header.Get("INCH-ACCESS-SIGN")
		passphrase := r.Header.Get("INCH-ACCESS-PASSPHRASE")

		if accessKey == "" || timestamp == "" || signature == "" || passphrase == "" {
			m.writeAuthError(w, "Missing required authentication headers")
			return
		}

		// Validate access key and passphrase
		if accessKey != m.config.OneInchAuth.AccessKey || passphrase != m.config.OneInchAuth.Passphrase {
			m.writeAuthError(w, "Invalid access key or passphrase")
			return
		}

		// Read and store body for signature verification
		var body []byte
		if r.Body != nil {
			var readErr error
			body, readErr = io.ReadAll(r.Body)
			if readErr != nil {
				m.writeAuthError(w, "Failed to read request body")
				return
			}
			// Create new reader for downstream handlers
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}

		// Generate expected signature
		expectedSignature, err := m.generateSignature(r.Method, r.URL.Path, r.URL.RawQuery, string(body), timestamp)
		if err != nil {
			m.writeAuthError(w, "Failed to generate signature")
			return
		}

		// Compare signatures
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			m.writeAuthError(w, "Invalid signature")
			return
		}

		// Authentication passed, continue to next handler
		next.ServeHTTP(w, r)
	})
}

func (m *OneInchAuthMiddleware) generateSignature(method, path, rawQuery, body, timestamp string) (string, error) {
	// Parse query parameters and sort them
	var sortedQuery string
	if rawQuery != "" {
		values, err := url.ParseQuery(rawQuery)
		if err != nil {
			return "", err
		}

		var pairs []string
		for key, vals := range values {
			for _, val := range vals {
				pairs = append(pairs, fmt.Sprintf("%s=%s", key, val))
			}
		}
		sort.Strings(pairs)
		sortedQuery = strings.Join(pairs, "&")
	}

	// Build payload: timestamp + method + path + query + sortedBody
	var payload strings.Builder
	payload.WriteString(timestamp)
	payload.WriteString(strings.ToUpper(method))
	payload.WriteString(path)
	if sortedQuery != "" {
		payload.WriteString("?")
		payload.WriteString(sortedQuery)
	}

	// For POST requests, add body (sorted if form data, as-is if JSON)
	if method == "POST" && body != "" {
		// According to 1inch spec, the body should be sorted as key=value pairs for form data
		// For JSON, we need to parse and sort the keys
		if strings.HasPrefix(strings.TrimSpace(body), "{") {
			// Parse JSON and sort keys for consistent signature
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(body), &jsonData); err == nil {
				var pairs []string
				for key, value := range jsonData {
					pairs = append(pairs, fmt.Sprintf("%s=%v", key, value))
				}
				sort.Strings(pairs)
				payload.WriteString(strings.Join(pairs, "&"))
			} else {
				// If JSON parsing fails, use body as-is
				payload.WriteString(body)
			}
		} else {
			// Form data - parse and sort
			values, err := url.ParseQuery(body)
			if err != nil {
				// If parsing fails, use body as-is
				payload.WriteString(body)
			} else {
				var pairs []string
				for key, vals := range values {
					for _, val := range vals {
						pairs = append(pairs, fmt.Sprintf("%s=%s", key, val))
					}
				}
				sort.Strings(pairs)
				payload.WriteString(strings.Join(pairs, "&"))
			}
		}
	}

	// Generate HMAC-SHA256 signature
	h := hmac.New(sha256.New, []byte(m.config.OneInchAuth.SecretKey))
	h.Write([]byte(payload.String()))

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *OneInchAuthMiddleware) writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusUnauthorized)

	errorResp := models.ErrorResponse{
		Error:   "Unauthorized",
		Message: message,
		Code:    http.StatusUnauthorized,
	}

	// Note: We're ignoring the encoding error here to avoid recursive error handling
	_ = json.NewEncoder(w).Encode(errorResp)
}
