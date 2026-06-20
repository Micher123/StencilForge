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
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		plan TEXT NOT NULL DEFAULT 'free',
		newsletter_opt_in INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`
	if _, err := DB.Exec(query); err != nil {
		return err
	}
	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}

// CreateUser inserts a new user. Returns the new user ID.
func CreateUser(username, email, passwordHash string, newsletterOptIn bool) (int64, error) {
	optIn := 0
	if newsletterOptIn {
		optIn = 1
	}
	result, err := DB.Exec(
		"INSERT INTO users (username, email, password_hash, plan, newsletter_opt_in, created_at) VALUES (?, ?, ?, 'free', ?, ?)",
		username, email, passwordHash, optIn, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

type User struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	PasswordHash    string `json:"-"`
	Plan            string `json:"plan"`
	NewsletterOptIn bool   `json:"newsletter_opt_in"`
	CreatedAt       string `json:"created_at"`
}

func GetUserByEmail(email string) (*User, error) {
	u := &User{}
	var optIn int
	err := DB.QueryRow(
		"SELECT id, username, email, password_hash, plan, newsletter_opt_in, created_at FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Plan, &optIn, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.NewsletterOptIn = optIn == 1
	return u, nil
}

func GetUserByID(id int64) (*User, error) {
	u := &User{}
	var optIn int
	err := DB.QueryRow(
		"SELECT id, username, email, password_hash, plan, newsletter_opt_in, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Plan, &optIn, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.NewsletterOptIn = optIn == 1
	return u, nil
}
