package middleware

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type AuthConfig struct {
	Enabled     bool
	APIPassword string
	APIKeyEnv   string
	PublicPaths []string
}

func Auth(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path is public
			for _, path := range config.PublicPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Get expected API key
			expectedKey := config.APIPassword
			if expectedKey == "" && config.APIKeyEnv != "" {
				expectedKey = os.Getenv(config.APIKeyEnv)
			}

			if expectedKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check Authorization header
			auth := r.Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")

			if token != expectedKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error": map[string]string{
						"code":    "UNAUTHORIZED",
						"message": "Invalid or missing API key",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
