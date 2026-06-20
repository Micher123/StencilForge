package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"stencilforge/auth"
	"stencilforge/db"
)

type RegisterRequest struct {
	Username        string `json:"username"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
	Newsletter      bool   `json:"newsletter"`
}

type RegisterResponse struct {
	OK    bool   `json:"ok"`
	Token string `json:"token,omitempty"`
	Error string `json:"error,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	OK    bool   `json:"ok"`
	Token string `json:"token,omitempty"`
	Error string `json:"error,omitempty"`
	User  *struct {
		ID              int64  `json:"id"`
		Username        string `json:"username"`
		Email           string `json:"email"`
		Plan            string `json:"plan"`
		NewsletterOptIn bool   `json:"newsletter_opt_in"`
	} `json:"user,omitempty"`
}

type MeResponse struct {
	OK   bool     `json:"ok"`
	User *db.User `json:"user,omitempty"`
}

// RegisterHandler обрабатывает POST /api/register
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{OK: false, Error: "Неверный формат запроса"})
		return
	}

	// Проверка обязательных полей
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = req.Password // пароль не тримим, пробелы могут быть частью пароля

	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{OK: false, Error: "Все поля обязательны для заполнения"})
		return
	}

	if req.Password != req.PasswordConfirm {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{OK: false, Error: "Пароли не совпадают"})
		return
	}

	if !auth.ValidateEmailDomain(req.Email) {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{OK: false, Error: "Некорректный формат email"})
		return
	}

	// Проверка сложности пароля
	if valid, msg := auth.CheckPasswordStrength(req.Password); !valid {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{OK: false, Error: msg})
		return
	}

	// Проверка, что пользователь с таким email уже существует
	existing, err := db.GetUserByEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, RegisterResponse{OK: false, Error: "Ошибка сервера"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, RegisterResponse{OK: false, Error: "Пользователь с таким email уже зарегистрирован"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, RegisterResponse{OK: false, Error: "Ошибка сервера"})
		return
	}

	userID, err := db.CreateUser(req.Username, req.Email, hash, req.Newsletter)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeJSON(w, http.StatusConflict, RegisterResponse{OK: false, Error: "Пользователь с таким email уже зарегистрирован"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, RegisterResponse{OK: false, Error: "Ошибка сервера"})
		return
	}

	token, err := auth.GenerateToken(userID, req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, RegisterResponse{OK: false, Error: "Ошибка сервера"})
		return
	}

	// Ставим cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // для localhost
		SameSite: http.SameSiteLaxMode,
		MaxAge:   72 * 3600,
	})

	writeJSON(w, http.StatusOK, RegisterResponse{OK: true, Token: token})
}

// LoginHandler обрабатывает POST /api/login
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, LoginResponse{OK: false, Error: "Неверный формат запроса"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, LoginResponse{OK: false, Error: "Email и пароль обязательны"})
		return
	}

	user, err := db.GetUserByEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, LoginResponse{OK: false, Error: "Ошибка сервера"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, LoginResponse{OK: false, Error: "Неверный email или пароль"})
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, LoginResponse{OK: false, Error: "Неверный email или пароль"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, LoginResponse{OK: false, Error: "Ошибка сервера"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   72 * 3600,
	})

	writeJSON(w, http.StatusOK, LoginResponse{
		OK:    true,
		Token: token,
		User: &struct {
			ID              int64  `json:"id"`
			Username        string `json:"username"`
			Email           string `json:"email"`
			Plan            string `json:"plan"`
			NewsletterOptIn bool   `json:"newsletter_opt_in"`
		}{
			ID:              user.ID,
			Username:        user.Username,
			Email:           user.Email,
			Plan:            user.Plan,
			NewsletterOptIn: user.NewsletterOptIn,
		},
	})
}

// MeHandler обрабатывает GET /api/me (информация о текущем пользователе)
func MeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		writeJSON(w, http.StatusUnauthorized, MeResponse{OK: false})
		return
	}

	var userID int64
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, MeResponse{OK: false})
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, MeResponse{OK: false})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, MeResponse{OK: false})
		return
	}

	writeJSON(w, http.StatusOK, MeResponse{OK: true, User: user})
}

// LogoutHandler обрабатывает POST /api/logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
