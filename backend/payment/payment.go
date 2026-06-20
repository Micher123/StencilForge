package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// YooKassaConfig содержит настройки для подключения к ЮKassa
type YooKassaConfig struct {
	ShopID    string
	SecretKey string
	ReturnURL string
}

// LoadConfig загружает конфигурацию из переменных окружения
func LoadConfig() *YooKassaConfig {
	return &YooKassaConfig{
		ShopID:    os.Getenv("YOOKASSA_SHOP_ID"),
		SecretKey: os.Getenv("YOOKASSA_SECRET_KEY"),
		ReturnURL: getEnvOrDefault("YOOKASSA_RETURN_URL", "http://localhost:8080/profile"),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// IsConfigured проверяет, настроена ли ЮKassa
func (c *YooKassaConfig) IsConfigured() bool {
	return c.ShopID != "" && c.SecretKey != ""
}

// CreatePaymentRequest — запрос на создание платежа
type CreatePaymentRequest struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Capture      bool `json:"capture"`
	Confirmation struct {
		Type      string `json:"type"`
		ReturnURL string `json:"return_url"`
	} `json:"confirmation"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CreatePaymentResponse — ответ от ЮKassa
type CreatePaymentResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Paid   bool   `json:"paid"`
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Confirmation struct {
		Type            string `json:"type"`
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
	CreatedAt   time.Time              `json:"created_at"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorResponse — ошибка от ЮKassa
type ErrorResponse struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Parameter   string `json:"parameter,omitempty"`
}

// CreatePayment создаёт платёж в ЮKassa и возвращает URL для оплаты
func (c *YooKassaConfig) CreatePayment(description string, amountKop int, metadata map[string]interface{}) (*CreatePaymentResponse, error) {
	amountRub := float64(amountKop) / 100.0
	req := CreatePaymentRequest{
		Capture:     true,
		Description: description,
		Metadata:    metadata,
	}
	req.Amount.Value = fmt.Sprintf("%.2f", amountRub)
	req.Amount.Currency = "RUB"
	req.Confirmation.Type = "redirect"
	req.Confirmation.ReturnURL = c.ReturnURL

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.yookassa.ru/v3/payments", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.SetBasicAuth(c.ShopID, c.SecretKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", generateIdempotenceKey())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil {
			return nil, fmt.Errorf("yookassa error: %s — %s", errResp.Code, errResp.Description)
		}
		return nil, fmt.Errorf("yookassa http %d: %s", resp.StatusCode, string(respBody))
	}

	var payResp CreatePaymentResponse
	if err := json.Unmarshal(respBody, &payResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &payResp, nil
}

// GetPaymentInfo получает информацию о платеже по ID
func (c *YooKassaConfig) GetPaymentInfo(paymentID string) (*CreatePaymentResponse, error) {
	httpReq, err := http.NewRequest("GET", "https://api.yookassa.ru/v3/payments/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.SetBasicAuth(c.ShopID, c.SecretKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("yookassa http %d: %s", resp.StatusCode, string(respBody))
	}

	var payResp CreatePaymentResponse
	if err := json.Unmarshal(respBody, &payResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &payResp, nil
}

func generateIdempotenceKey() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
