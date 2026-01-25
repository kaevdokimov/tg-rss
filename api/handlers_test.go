package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid HTTP URL",
			url:     "http://example.com/feed.xml",
			wantErr: false,
		},
		{
			name:    "Valid HTTPS URL",
			url:     "https://example.com/feed.xml",
			wantErr: false,
		},
		{
			name:    "Empty URL",
			url:     "",
			wantErr: true,
			errMsg:  "URL не может быть пустым",
		},
		{
			name:    "Too long URL",
			url:     "https://example.com/" + string(make([]byte, 2100)),
			wantErr: true,
			errMsg:  "URL слишком длинный",
		},
		{
			name:    "Invalid scheme",
			url:     "ftp://example.com/feed.xml",
			wantErr: true,
			errMsg:  "разрешены только http и https схемы",
		},
		{
			name:    "Localhost forbidden",
			url:     "http://localhost:8080/feed",
			wantErr: true,
			errMsg:  "запрещено использовать внутренние/локальные адреса",
		},
		{
			name:    "127.0.0.1 forbidden",
			url:     "http://127.0.0.1/feed",
			wantErr: true,
			errMsg:  "запрещено использовать внутренние/локальные адреса",
		},
		{
			name:    "Private IP 192.168 forbidden",
			url:     "http://192.168.1.1/feed",
			wantErr: true,
			errMsg:  "запрещено использовать внутренние/локальные адреса",
		},
		{
			name:    "No host",
			url:     "http:///feed",
			wantErr: true,
			errMsg:  "URL должен содержать хост",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid name",
			input:   "Test Source",
			wantErr: false,
		},
		{
			name:    "Empty name",
			input:   "",
			wantErr: true,
			errMsg:  "имя не может быть пустым",
		},
		{
			name:    "Too long name",
			input:   string(make([]byte, 300)),
			wantErr: true,
			errMsg:  "имя слишком длинное",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSendJSON(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		response       APIResponse
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "Success response",
			status: http.StatusOK,
			response: APIResponse{
				Success: true,
				Data:    map[string]string{"message": "ok"},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"success":true,"data":{"message":"ok"}}`,
		},
		{
			name:   "Error response",
			status: http.StatusBadRequest,
			response: APIResponse{
				Success: false,
				Error:   "validation failed",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"success":false,"error":"validation failed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			sendJSON(w, tt.status, tt.response)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestCreateSourceHandler_Validation(t *testing.T) {
	// Не используем реальную БД, проверяем только валидацию
	handler := CreateSourceHandler(nil)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid JSON",
			requestBody:    "not-json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON format",
		},
		{
			name: "Empty name",
			requestBody: CreateSourceRequest{
				Name: "",
				URL:  "https://example.com/feed",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "имя не может быть пустым",
		},
		{
			name: "Invalid URL scheme",
			requestBody: CreateSourceRequest{
				Name: "Test Source",
				URL:  "ftp://example.com/feed",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "разрешены только http и https схемы",
		},
		{
			name: "Localhost URL",
			requestBody: CreateSourceRequest{
				Name: "Test Source",
				URL:  "http://localhost/feed",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "запрещено использовать внутренние/локальные адреса",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/create", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response APIResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.False(t, response.Success)
			assert.Contains(t, response.Error, tt.expectedError)
		})
	}
}

func TestUpdateSourceHandler_Validation(t *testing.T) {
	// Пропускаем тесты, которые требуют реальную БД
	handler := UpdateSourceHandler(nil)

	tests := []struct {
		name           string
		method         string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
		skipDBCheck    bool
	}{
		{
			name:           "Wrong method",
			method:         http.MethodGet,
			requestBody:    UpdateSourceRequest{},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
			skipDBCheck:    true,
		},
		{
			name:           "Invalid JSON",
			method:         http.MethodPut,
			requestBody:    "not-json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON format",
			skipDBCheck:    true,
		},
		// Пропускаем тесты с валидацией URL, так как они требуют БД
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.skipDBCheck {
				t.Skip("Требуется реальная БД")
			}

			var body []byte
			var err error

			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, "/api/v1/sources/update?id=1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response APIResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.False(t, response.Success)
			assert.Contains(t, response.Error, tt.expectedError)
		})
	}
}

func TestGetUserHandler_InvalidInput(t *testing.T) {
	handler := GetUserHandler(nil)

	tests := []struct {
		name           string
		queryParam     string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Missing chat_id",
			queryParam:     "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "chat_id parameter is required",
		},
		{
			name:           "Invalid chat_id format",
			queryParam:     "abc",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid chat_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/users/check"
			if tt.queryParam != "" {
				url += "?chat_id=" + tt.queryParam
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.False(t, response.Success)
			assert.Contains(t, response.Error, tt.expectedError)
		})
	}
}

// MockDB для тестирования (можно расширить при необходимости)
type MockDB struct {
	*sql.DB
}

func TestAPIResponse_Structure(t *testing.T) {
	// Проверяем структуру ответов API
	t.Run("Success response structure", func(t *testing.T) {
		resp := APIResponse{
			Success: true,
			Data:    map[string]int{"count": 5},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var decoded map[string]interface{}
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.True(t, decoded["success"].(bool))
		assert.NotNil(t, decoded["data"])
	})

	t.Run("Error response structure", func(t *testing.T) {
		resp := APIResponse{
			Success: false,
			Error:   "test error",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var decoded map[string]interface{}
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.False(t, decoded["success"].(bool))
		assert.Equal(t, "test error", decoded["error"])
	})
}

func TestSourceResponse_Structure(t *testing.T) {
	source := SourceResponse{
		ID:        1,
		Name:      "Test Source",
		URL:       "https://example.com/feed",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(source)
	require.NoError(t, err)

	var decoded SourceResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, source.ID, decoded.ID)
	assert.Equal(t, source.Name, decoded.Name)
	assert.Equal(t, source.URL, decoded.URL)
	assert.Equal(t, source.Status, decoded.Status)
}
