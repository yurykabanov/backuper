package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/yurykabanov/backuper/pkg/appcontext"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func WithRequestLogging(next http.Handler, logger logrus.FieldLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := time.Now()
		sw := statusWriter{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			logger := appcontext.LoggerFromContext(logger, r.Context())

			logger.WithFields(logrus.Fields{
				"host":           r.Host,
				"remote_addr":    r.RemoteAddr,
				"method":         r.Method,
				"request_uri":    r.RequestURI,
				"status":         sw.status,
				"content_length": sw.length,
				"user_agent":     r.UserAgent(),
				"duration_ns":    time.Now().Sub(startAt).Nanoseconds(),
			}).Info("request")
		}()

		next.ServeHTTP(&sw, r)
	})
}
