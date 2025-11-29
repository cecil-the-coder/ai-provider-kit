package middleware

import (
    "encoding/json"
    "log"
    "net/http"
    "runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                requestID := GetRequestID(r.Context())
                log.Printf("[%s] PANIC: %v\n%s", requestID, err, debug.Stack())

                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusInternalServerError)
                _ = json.NewEncoder(w).Encode(map[string]interface{}{
                    "success": false,
                    "error": map[string]string{
                        "code":    "INTERNAL_ERROR",
                        "message": "An internal error occurred",
                    },
                })
            }
        }()

        next.ServeHTTP(w, r)
    })
}
