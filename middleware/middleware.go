package middleware

import (
	"context"
	"net/http"
	"time"

	"tg-rss/monitoring"
)

// Middleware представляет функцию middleware
type Middleware func(http.HandlerFunc) http.HandlerFunc

// Chain применяет цепочку middleware к обработчику
func Chain(handler http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// CORS добавляет CORS заголовки
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// Recovery перехватывает паники и восстанавливает приложение
func Recovery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				monitoring.GetLogger("http").Error("HTTP panic recovered",
					"panic", err,
					"url", r.URL.Path,
					"method", r.Method,
					"remote_addr", r.RemoteAddr,
				)

				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			}
		}()

		next(w, r)
	}
}

// Timeout добавляет таймаут для запросов
func Timeout(timeout time.Duration) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)

			done := make(chan struct{})
			go func() {
				next(w, r)
				close(done)
			}()

			select {
			case <-done:
				// Запрос завершен успешно
			case <-ctx.Done():
				// Таймаут
				monitoring.GetLogger("http").Warn("HTTP request timeout",
					"url", r.URL.Path,
					"method", r.Method,
					"remote_addr", r.RemoteAddr,
					"timeout", timeout,
				)

				if w.Header().Get("Content-Type") == "" {
					w.WriteHeader(http.StatusGatewayTimeout)
					w.Write([]byte("Request timeout"))
				}
			}
		}
	}
}

// Logging логирует HTTP запросы
func Logging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Обертываем ResponseWriter для захвата статуса
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next(rw, r)

		duration := time.Since(start)

		monitoring.GetLogger("http").LogHTTPRequest(
			r.Method,
			r.URL.Path,
			http.StatusText(rw.statusCode),
			int64(duration/time.Millisecond),
			rw.size,
		)
	}
}

// responseWriter обертывает http.ResponseWriter для захвата статуса и размера ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(data)
	rw.size += int64(size)
	return size, err
}