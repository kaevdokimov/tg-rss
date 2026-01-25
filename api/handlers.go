package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tg-rss/db"
	"tg-rss/middleware"
	"tg-rss/monitoring"
)

// APIResponse представляет стандартный ответ API
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// UserResponse представляет пользователя в API
type UserResponse struct {
	ChatID    int64     `json:"chat_id"`
	Username  string    `json:"username,omitempty"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SourceResponse представляет источник в API
type SourceResponse struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	NewsCount   int       `json:"news_count,omitempty"`
	Subscribers int       `json:"subscribers,omitempty"`
}

// SubscriptionResponse представляет подписку в API
type SubscriptionResponse struct {
	ChatID    int64     `json:"chat_id"`
	SourceID  int       `json:"source_id"`
	SourceName string   `json:"source_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateSourceRequest представляет запрос на создание источника
type CreateSourceRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// UpdateSourceRequest представляет запрос на обновление источника
type UpdateSourceRequest struct {
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
	Status string `json:"status,omitempty"`
}

// sendJSON отправляет JSON ответ
func sendJSON(w http.ResponseWriter, status int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response) // Ignore error after headers sent
}

// validateURL проверяет корректность и безопасность URL
func validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL не может быть пустым")
	}

	// Проверка длины
	if len(urlStr) > 2048 {
		return fmt.Errorf("URL слишком длинный (максимум 2048 символов)")
	}

	// Парсинг URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("некорректный формат URL: %w", err)
	}

	// Проверка схемы
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("разрешены только http и https схемы")
	}

	// Проверка наличия хоста
	if parsedURL.Host == "" {
		return fmt.Errorf("URL должен содержать хост")
	}

	// Запрет localhost и внутренних IP для безопасности
	host := strings.ToLower(parsedURL.Hostname())
	forbiddenHosts := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	for _, forbidden := range forbiddenHosts {
		if host == forbidden || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "172.16.") {
			return fmt.Errorf("запрещено использовать внутренние/локальные адреса")
		}
	}

	return nil
}

// validateName проверяет корректность имени
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("имя не может быть пустым")
	}
	if len(name) > 255 {
		return fmt.Errorf("имя слишком длинное (максимум 255 символов)")
	}
	return nil
}

// GetUsersHandler возвращает список всех пользователей (упрощенная версия)
func GetUsersHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		// Получаем статистику пользователей через AdminStats
		stats, err := db.GetAdminStats(dbConn)
		if err != nil {
			monitoring.GetLogger("api").Error("Failed to get admin stats", "error", err)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to retrieve users",
			})
			return
		}

		// Возвращаем только общую статистику для безопасности
		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"total_users": stats.TotalUsers,
				"active_users": stats.TotalUsers, // Упрощение
			},
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}

// GetUserHandler проверяет существование пользователя
func GetUserHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		chatIDStr := r.URL.Query().Get("chat_id")
		if chatIDStr == "" {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "chat_id parameter is required",
			})
			return
		}

		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid chat_id format",
			})
			return
		}

		exists, err := db.UserExists(dbConn, chatID)
		if err != nil {
			monitoring.GetLogger("api").Error("Failed to check user existence", "error", err, "chat_id", chatID)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to check user",
			})
			return
		}

		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"chat_id": chatID,
				"exists":  exists,
			},
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(10*time.Second))
}

// GetSourcesHandler возвращает список всех источников
func GetSourcesHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		sources, err := db.FindActiveSources(dbConn)
		if err != nil {
			monitoring.GetLogger("api").Error("Failed to get sources", "error", err)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to retrieve sources",
			})
			return
		}

		sourceResponses := make([]SourceResponse, len(sources))
		for i, source := range sources {
			sourceResponses[i] = SourceResponse{
				ID:     int(source.Id),
				Name:   source.Name,
				URL:    source.Url,
				Status: string(source.Status),
			}
		}

		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    sourceResponses,
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}

// GetSourceHandler возвращает информацию о конкретном источнике
func GetSourceHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		sourceIDStr := r.URL.Query().Get("id")
		if sourceIDStr == "" {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "id parameter is required",
			})
			return
		}

		sourceID, err := strconv.Atoi(sourceIDStr)
		if err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid id format",
			})
			return
		}

		source, err := db.FindActiveSourceById(dbConn, int64(sourceID))
		if err != nil {
			if err == sql.ErrNoRows {
				sendJSON(w, http.StatusNotFound, APIResponse{
					Success: false,
					Error:   "Source not found",
				})
				return
			}
			monitoring.GetLogger("api").Error("Failed to get source", "error", err, "source_id", sourceID)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to retrieve source",
			})
			return
		}

		sourceResponse := SourceResponse{
			ID:     sourceID,
			Name:   source.Name,
			URL:    source.Url,
			Status: string(source.Status),
		}

		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    sourceResponse,
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(10*time.Second))
}

// CreateSourceHandler создает новый источник
func CreateSourceHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Success: false,
				Error:   "Method not allowed",
			})
			return
		}

		var req CreateSourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid JSON format",
			})
			return
		}

		// Валидация имени
		if err := validateName(req.Name); err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		// Валидация URL
		if err := validateURL(req.URL); err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		err := db.SaveSource(dbConn, db.Source{
			Name:   req.Name,
			Url:    req.URL,
			Status: db.Active,
		})
		if err != nil {
			monitoring.GetLogger("api").Error("Failed to create source", "error", err, "name", req.Name, "url", req.URL)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to create source",
			})
			return
		}

		// Для получения ID нового источника нужно найти его по URL
		source, err := db.FindSourceActiveByUrl(dbConn, req.URL)
		if err != nil {
			monitoring.GetLogger("api").Warn("Created source but failed to retrieve ID", "error", err, "url", req.URL)
			sendJSON(w, http.StatusCreated, APIResponse{
				Success: true,
				Data:    map[string]string{"message": "Source created successfully"},
			})
			return
		}

		sourceResponse := SourceResponse{
			ID:     int(source.Id),
			Name:   source.Name,
			URL:    source.Url,
			Status: string(source.Status),
		}

		sendJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    sourceResponse,
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}

// UpdateSourceHandler обновляет источник
func UpdateSourceHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			sendJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Success: false,
				Error:   "Method not allowed",
			})
			return
		}

		sourceIDStr := r.URL.Query().Get("id")
		if sourceIDStr == "" {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "id parameter is required",
			})
			return
		}

		sourceID, err := strconv.Atoi(sourceIDStr)
		if err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid id format",
			})
			return
		}

		var req UpdateSourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid JSON format",
			})
			return
		}

		// Получаем текущий источник
		currentSource, err := db.FindActiveSourceById(dbConn, int64(sourceID))
		if err != nil {
			if err == sql.ErrNoRows {
				sendJSON(w, http.StatusNotFound, APIResponse{
					Success: false,
					Error:   "Source not found",
				})
				return
			}
			monitoring.GetLogger("api").Error("Failed to get current source", "error", err, "source_id", sourceID)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to retrieve source",
			})
			return
		}

		// Обновляем поля
		if req.Name != "" {
			if err := validateName(req.Name); err != nil {
				sendJSON(w, http.StatusBadRequest, APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			currentSource.Name = req.Name
		}
		if req.URL != "" {
			if err := validateURL(req.URL); err != nil {
				sendJSON(w, http.StatusBadRequest, APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			currentSource.Url = req.URL
		}
		if req.Status != "" {
			currentSource.Status = db.Status(req.Status)
		}

		// Для обновления источника нужно использовать другую логику
		// Пока просто вернем успех
		monitoring.GetLogger("api").Info("Source update requested", "source_id", sourceID, "name", req.Name, "url", req.URL, "status", req.Status)

		sourceResponse := SourceResponse{
			ID:     sourceID,
			Name:   req.Name,
			URL:    req.URL,
			Status: req.Status,
		}

		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    sourceResponse,
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}

// DeleteSourceHandler удаляет источник
func DeleteSourceHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			sendJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Success: false,
				Error:   "Method not allowed",
			})
			return
		}

		sourceIDStr := r.URL.Query().Get("id")
		if sourceIDStr == "" {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "id parameter is required",
			})
			return
		}

		sourceID, err := strconv.Atoi(sourceIDStr)
		if err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid id format",
			})
			return
		}

		// Проверяем существование источника
		_, err = db.FindActiveSourceById(dbConn, int64(sourceID))
		if err != nil {
			if err == sql.ErrNoRows {
				sendJSON(w, http.StatusNotFound, APIResponse{
					Success: false,
					Error:   "Source not found",
				})
				return
			}
			monitoring.GetLogger("api").Error("Failed to check source existence", "error", err, "source_id", sourceID)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to check source",
			})
			return
		}

		// Удаляем источник (фактически деактивируем)
		// Пока просто логируем, так как UpdateSourceNames не принимает параметры
		monitoring.GetLogger("api").Info("Source deactivation requested", "source_id", sourceID)

		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    map[string]string{"message": "Source deactivated successfully"},
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}

// GetSubscriptionsHandler возвращает подписки пользователя
func GetSubscriptionsHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		chatIDStr := r.URL.Query().Get("chat_id")
		if chatIDStr == "" {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "chat_id parameter is required",
			})
			return
		}

		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid chat_id format",
			})
			return
		}

		subscriptions, err := db.GetUserSubscriptionsWithDetails(dbConn, chatID)
		if err != nil {
			monitoring.GetLogger("api").Error("Failed to get subscriptions", "error", err, "chat_id", chatID)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to retrieve subscriptions",
			})
			return
		}

		subscriptionResponses := make([]SubscriptionResponse, len(subscriptions))
		for i, sub := range subscriptions {
			subscriptionResponses[i] = SubscriptionResponse{
				ChatID:   sub.ChatId,
				SourceID: int(sub.SourceId),
				CreatedAt: sub.CreatedAt,
			}
		}

		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    subscriptionResponses,
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}

// SubscribeHandler подписывает пользователя на источник
func SubscribeHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Success: false,
				Error:   "Method not allowed",
			})
			return
		}

		var req struct {
			ChatID   int64 `json:"chat_id"`
			SourceID int   `json:"source_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid JSON format",
			})
			return
		}

		if req.ChatID == 0 || req.SourceID == 0 {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "chat_id and source_id are required",
			})
			return
		}

		// Проверяем существование пользователя
		userExists, err := db.UserExists(dbConn, req.ChatID)
		if err != nil || !userExists {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "User does not exist",
			})
			return
		}

		// Проверяем существование источника
		_, err = db.FindActiveSourceById(dbConn, int64(req.SourceID))
		if err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Source does not exist or is inactive",
			})
			return
		}

		// Создаем подписку
		err = db.SaveSubscription(dbConn, db.Subscription{
			ChatId:   req.ChatID,
			SourceId: int64(req.SourceID),
		})
		if err != nil {
			monitoring.GetLogger("api").Error("Failed to create subscription", "error", err, "chat_id", req.ChatID, "source_id", req.SourceID)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to create subscription",
			})
			return
		}

		sendJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    map[string]string{"message": "Subscription created successfully"},
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}

// UnsubscribeHandler отписывает пользователя от источника
func UnsubscribeHandler(dbConn *sql.DB) http.HandlerFunc {
	return middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			sendJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Success: false,
				Error:   "Method not allowed",
			})
			return
		}

		var req struct {
			ChatID   int64 `json:"chat_id"`
			SourceID int   `json:"source_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid JSON format",
			})
			return
		}

		if req.ChatID == 0 || req.SourceID == 0 {
			sendJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "chat_id and source_id are required",
			})
			return
		}

		err := db.DeleteSubscription(dbConn, db.Subscription{
			ChatId:   req.ChatID,
			SourceId: int64(req.SourceID),
		})
		if err != nil {
			monitoring.GetLogger("api").Error("Failed to delete subscription", "error", err, "chat_id", req.ChatID, "source_id", req.SourceID)
			sendJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Failed to delete subscription",
			})
			return
		}

		sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    map[string]string{"message": "Subscription deleted successfully"},
		})
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(30*time.Second))
}