package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "stencilforge.db")
	var err error
	DB, err = sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	DB.SetMaxOpenConns(1) // SQLite single-writer

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

func migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			plan TEXT NOT NULL DEFAULT 'free',
			max_layers INTEGER NOT NULL DEFAULT 3,
			newsletter_opt_in INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);`,
		`CREATE TABLE IF NOT EXISTS payments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			payment_id TEXT NOT NULL UNIQUE,
			plan TEXT NOT NULL,
			duration TEXT NOT NULL DEFAULT '1m',
			amount_rub INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			confirmed_at TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`,
	}
	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}
	// Ensure max_layers column exists for older DBs
	DB.Exec("ALTER TABLE users ADD COLUMN max_layers INTEGER NOT NULL DEFAULT 3")
	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}

// PlanLimits возвращает лимит слоёв для тарифа
func PlanLimits(plan string) int {
	switch plan {
	case "ultima":
		return 32
	case "pro":
		return 10
	default:
		return 3
	}
}

// PlanPriceRub возвращает цену тарифа в рублях (целое число) для указанной длительности
func PlanPriceRub(plan string, duration string) int {
	switch plan {
	case "ultima":
		switch duration {
		case "1m":
			return 499
		case "3m":
			return 1099
		case "12m":
			return 3999
		default:
			return 499
		}
	case "pro":
		switch duration {
		case "1m":
			return 299
		case "3m":
			return 799
		case "12m":
			return 2999
		default:
			return 299
		}
	default:
		return 0
	}
}

// PlanPriceKop возвращает цену тарифа в копейках (для ЮKassa)
func PlanPriceKop(plan string, duration string) int {
	return PlanPriceRub(plan, duration) * 100
}

// UpgradePlan меняет тариф пользователю
func UpgradePlan(userID int64, plan string, maxLayers int) error {
	_, err := DB.Exec("UPDATE users SET plan = ?, max_layers = ? WHERE id = ?", plan, maxLayers, userID)
	return err
}

// SavePayment сохраняет запись о платеже
func SavePayment(userID int64, paymentID string, plan string, duration string, amountRub int) error {
	_, err := DB.Exec(
		"INSERT INTO payments (user_id, payment_id, plan, duration, amount_rub, status) VALUES (?, ?, ?, ?, ?, 'pending')",
		userID, paymentID, plan, duration, amountRub,
	)
	return err
}

// ConfirmPayment подтверждает платёж
func ConfirmPayment(paymentID string) error {
	_, err := DB.Exec(
		"UPDATE payments SET status = 'succeeded', confirmed_at = datetime('now') WHERE payment_id = ?",
		paymentID,
	)
	return err
}

// Payment represents a payment record
type Payment struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	PaymentID   string `json:"payment_id"`
	Plan        string `json:"plan"`
	Duration    string `json:"duration"`
	AmountRub   int    `json:"amount_rub"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	ConfirmedAt string `json:"confirmed_at,omitempty"`
}

// GetPayment возвращает платёж по payment_id
func GetPayment(paymentID string) (*Payment, error) {
	p := &Payment{}
	var confirmedAt sql.NullString
	err := DB.QueryRow(
		"SELECT id, user_id, payment_id, plan, duration, amount_rub, status, created_at, confirmed_at FROM payments WHERE payment_id = ?",
		paymentID,
	).Scan(&p.ID, &p.UserID, &p.PaymentID, &p.Plan, &p.Duration, &p.AmountRub, &p.Status, &p.CreatedAt, &confirmedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if confirmedAt.Valid {
		p.ConfirmedAt = confirmedAt.String
	}
	return p, nil
}

// CreateUser inserts a new user. Returns the new user ID.
func CreateUser(username, email, passwordHash string, newsletterOptIn bool) (int64, error) {
	optIn := 0
	if newsletterOptIn {
		optIn = 1
	}
	result, err := DB.Exec(
		"INSERT INTO users (username, email, password_hash, plan, max_layers, newsletter_opt_in, created_at) VALUES (?, ?, ?, 'free', 3, ?, ?)",
		username, email, passwordHash, optIn, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// User represents a registered user
type User struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	PasswordHash    string `json:"-"`
	Plan            string `json:"plan"`
	MaxLayers       int    `json:"max_layers"`
	NewsletterOptIn bool   `json:"newsletter_opt_in"`
	CreatedAt       string `json:"created_at"`
}

// GetUserByEmail returns a user by email, or nil if not found
func GetUserByEmail(email string) (*User, error) {
	u := &User{}
	var optIn int
	err := DB.QueryRow(
		"SELECT id, username, email, password_hash, plan, max_layers, newsletter_opt_in, created_at FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Plan, &u.MaxLayers, &optIn, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.NewsletterOptIn = optIn == 1
	return u, nil
}

// GetUserByID returns a user by ID, or nil if not found
func GetUserByID(id int64) (*User, error) {
	u := &User{}
	var optIn int
	err := DB.QueryRow(
		"SELECT id, username, email, password_hash, plan, max_layers, newsletter_opt_in, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Plan, &u.MaxLayers, &optIn, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.NewsletterOptIn = optIn == 1
	return u, nil
}
