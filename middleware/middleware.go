package middleware

import (
	"context"
	"net/http"
	"sync"
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

// CORS добавляет CORS заголовки с ограничениями
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Разрешаем только localhost для development или доверенные домены
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
		
		originAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				originAllowed = true
				break
			}
		}
		
		if originAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600")

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
				_, _ = w.Write([]byte("Internal Server Error"))
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
					_, _ = w.Write([]byte("Request timeout"))
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

// APIRateLimiter реализует rate limiting для API endpoints
type APIRateLimiter struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	limit    int           // максимум запросов
	window   time.Duration // временное окно
}

// NewAPIRateLimiter создает новый rate limiter
func NewAPIRateLimiter(limit int, window time.Duration) *APIRateLimiter {
	rl := &APIRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	// Запускаем фоновую очистку старых записей
	go rl.cleanup()
	return rl
}

// cleanup периодически очищает старые записи
func (rl *APIRateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, times := range rl.requests {
			// Удаляем старые запросы
			validTimes := []time.Time{}
			for _, t := range times {
				if now.Sub(t) < rl.window {
					validTimes = append(validTimes, t)
				}
			}
			if len(validTimes) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = validTimes
			}
		}
		rl.mu.Unlock()
	}
}

// isAllowed проверяет, разрешен ли запрос для данного IP
func (rl *APIRateLimiter) isAllowed(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	
	// Получаем историю запросов для этого IP
	times, exists := rl.requests[ip]
	if !exists {
		rl.requests[ip] = []time.Time{now}
		return true
	}

	// Фильтруем запросы в пределах окна
	validTimes := []time.Time{}
	for _, t := range times {
		if now.Sub(t) < rl.window {
			validTimes = append(validTimes, t)
		}
	}

	// Проверяем лимит
	if len(validTimes) >= rl.limit {
		return false
	}

	// Добавляем текущий запрос
	validTimes = append(validTimes, now)
	rl.requests[ip] = validTimes
	return true
}

// RateLimit создает middleware для rate limiting
func (rl *APIRateLimiter) RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// Извлекаем только IP без порта
		if idx := len(ip) - 1; idx >= 0 {
			for i := len(ip) - 1; i >= 0; i-- {
				if ip[i] == ':' {
					ip = ip[:i]
					break
				}
			}
		}

		if !rl.isAllowed(ip) {
			monitoring.GetLogger("http").Warn("Rate limit exceeded",
				"ip", ip,
				"url", r.URL.Path,
				"method", r.Method,
			)
			
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("Слишком много запросов. Попробуйте позже."))
			return
		}

		next(w, r)
	}
}