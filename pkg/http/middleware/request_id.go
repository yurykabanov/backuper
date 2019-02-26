package middleware

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/yurykabanov/backuper/pkg/appcontext"
)

func WithRequestId(next http.Handler, nextRequestId func() string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestId := r.Header.Get("X-Request-Id")

		if requestId == "" {
			requestId = nextRequestId()
		}

		ctx := appcontext.WithRequestId(r.Context(), requestId)

		w.Header().Set("X-Request-Id", requestId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func DefaultRequestIdProvider() string {
	var buf = make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%02x", buf)
}
