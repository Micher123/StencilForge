package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"stencilforge/db"
	"stencilforge/payment"
)

type CreatePaymentRequest struct {
	Plan     string `json:"plan"`
	Duration string `json:"duration"`
}

type CreatePaymentResponse struct {
	OK         bool   `json:"ok"`
	PaymentURL string `json:"payment_url,omitempty"`
	PaymentID  string `json:"payment_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

type PlansResponse struct {
	OK    bool       `json:"ok"`
	Plans []PlanInfo `json:"plans"`
}

type PlanDuration struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	PriceRub int    `json:"price_rub"`
}

type PlanInfo struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	MaxLayers int            `json:"max_layers"`
	Durations []PlanDuration `json:"durations"`
}

var plans = []PlanInfo{
	{
		ID: "free", Name: "Free", MaxLayers: 3,
		Durations: []PlanDuration{},
	},
	{
		ID: "pro", Name: "Pro", MaxLayers: 10,
		Durations: []PlanDuration{
			{ID: "1m", Name: "1 месяц", PriceRub: 299},
			{ID: "3m", Name: "3 месяца", PriceRub: 799},
			{ID: "12m", Name: "12 месяцев", PriceRub: 2999},
		},
	},
	{
		ID: "ultima", Name: "Ultima", MaxLayers: 16,
		Durations: []PlanDuration{
			{ID: "1m", Name: "1 месяц", PriceRub: 499},
			{ID: "3m", Name: "3 месяца", PriceRub: 1099},
			{ID: "12m", Name: "12 месяцев", PriceRub: 3999},
		},
	},
}

// PlansHandler возвращает список тарифов с длительностями (GET /api/plans)
func PlansHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}
	writeJSON(w, http.StatusOK, PlansResponse{OK: true, Plans: plans})
}

// CreatePaymentHandler создаёт платёж в ЮKassa (POST /api/create-payment)
func CreatePaymentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		writeJSON(w, http.StatusUnauthorized, CreatePaymentResponse{OK: false, Error: "Требуется авторизация. Пожалуйста, войдите в аккаунт."})
		return
	}

	var userID int64
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		writeJSON(w, http.StatusBadRequest, CreatePaymentResponse{OK: false, Error: "Некорректный идентификатор пользователя"})
		return
	}

	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, CreatePaymentResponse{OK: false, Error: "Неверный формат запроса. Пожалуйста, обновите страницу и попробуйте снова."})
		return
	}

	if req.Plan != "pro" && req.Plan != "ultima" {
		writeJSON(w, http.StatusBadRequest, CreatePaymentResponse{OK: false, Error: "Выбран несуществующий тарифный план"})
		return
	}

	// Валидация duration
	validDurations := map[string]bool{"1m": true, "3m": true, "12m": true}
	if !validDurations[req.Duration] {
		writeJSON(w, http.StatusBadRequest, CreatePaymentResponse{OK: false, Error: "Выбрана некорректная длительность подписки"})
		return
	}

	// Проверяем что пользователь существует
	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		writeJSON(w, http.StatusInternalServerError, CreatePaymentResponse{OK: false, Error: "Пользователь не найден. Пожалуйста, войдите заново."})
		return
	}

	// Если уже такой же тариф — не даём платить повторно
	if user.Plan == req.Plan {
		writeJSON(w, http.StatusBadRequest, CreatePaymentResponse{OK: false, Error: "у вас уже подключен этот тариф"})
		return
	}

	cfg := payment.LoadConfig()
	if !cfg.IsConfigured() {
		writeJSON(w, http.StatusServiceUnavailable, CreatePaymentResponse{OK: false, Error: "платёжная система не настроена"})
		return
	}

	amountKop := db.PlanPriceKop(req.Plan, req.Duration)
	amountRub := db.PlanPriceRub(req.Plan, req.Duration)
	description := fmt.Sprintf("StencilForge — %s тариф, %s", req.Plan, durationLabel(req.Duration))

	metadata := map[string]interface{}{
		"user_id":  userID,
		"plan":     req.Plan,
		"duration": req.Duration,
	}

	payResp, err := cfg.CreatePayment(description, amountKop, metadata)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, CreatePaymentResponse{OK: false, Error: "Не удалось создать платёж. Пожалуйста, попробуйте позже."})
		return
	}

	// Сохраняем платёж в БД
	if err := db.SavePayment(userID, payResp.ID, req.Plan, req.Duration, amountRub); err != nil {
		writeJSON(w, http.StatusInternalServerError, CreatePaymentResponse{OK: false, Error: "Не удалось сохранить платёж. Пожалуйста, попробуйте позже."})
		return
	}

	writeJSON(w, http.StatusOK, CreatePaymentResponse{
		OK:         true,
		PaymentURL: payResp.Confirmation.ConfirmationURL,
		PaymentID:  payResp.ID,
	})
}

// PaymentWebhookHandler обрабатывает уведомления от ЮKassa (POST /api/payment-webhook)
func PaymentWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}

	var notification struct {
		Type   string `json:"type"`
		Event  string `json:"event"`
		Object struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Paid     bool   `json:"paid"`
			Metadata struct {
				UserID float64 `json:"user_id"`
				Plan   string  `json:"plan"`
			} `json:"metadata"`
		} `json:"object"`
	}

	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Обрабатываем только успешные платежи
	if notification.Event == "payment.succeeded" && notification.Object.Status == "succeeded" && notification.Object.Paid {
		paymentID := notification.Object.ID
		plan := notification.Object.Metadata.Plan
		userID := int64(notification.Object.Metadata.UserID)

		// Подтверждаем в БД
		if err := db.ConfirmPayment(paymentID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Внутренняя ошибка сервера"})
			return
		}

		// Апгрейдим пользователя
		maxLayers := db.PlanLimits(plan)
		if err := db.UpgradePlan(userID, plan, maxLayers); err != nil {
			http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

// CheckPaymentHandler проверяет статус платежа и апгрейдит если succeeded (GET /api/check-payment?id=...)
func CheckPaymentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}

	paymentID := r.URL.Query().Get("id")
	if paymentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Не указан идентификатор платежа"})
		return
	}

	// Сначала проверяем локально
	p, err := db.GetPayment(paymentID)
	if err != nil || p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Платёж не найден"})
		return
	}

	if p.Status == "succeeded" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "status": "succeeded", "plan": p.Plan})
		return
	}

	// Проверяем в ЮKassa
	cfg := payment.LoadConfig()
	if cfg.IsConfigured() {
		payResp, err := cfg.GetPaymentInfo(paymentID)
		if err == nil && payResp.Status == "succeeded" && payResp.Paid {
			db.ConfirmPayment(paymentID)
			userID := p.UserID
			plan := p.Plan
			maxLayers := db.PlanLimits(plan)
			db.UpgradePlan(userID, plan, maxLayers)

			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "status": "succeeded", "plan": plan})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "status": p.Status, "plan": p.Plan})
}

func durationLabel(d string) string {
	switch d {
	case "1m":
		return "1 месяц"
	case "3m":
		return "3 месяца"
	case "12m":
		return "12 месяцев"
	default:
		return d
	}
}
