package credits

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// 1 device × 1 hour = 1 credit
// Credits convert to proxy GB at configurable rate
const (
	CreditsPerHour     = 100.0
	DefaultGBPerCredit = 0.001 // 1000 credits = 1 GB
	UptimeMultiplier24 = 1.5 // 24h+ streak bonus
	UptimeMultiplier72 = 2.0 // 72h+ streak bonus
	MultiDeviceBonus   = 0.2 // +20% per extra device
)

type CreditService struct {
	db *sql.DB
	mu sync.Mutex
}

type UserBalance struct {
	UserID         string  `json:"user_id"`
	Credits        float64 `json:"credits"`
	TotalEarned    float64 `json:"total_earned"`
	TotalSpent     float64 `json:"total_spent"`
	ActiveDevices  int     `json:"active_devices"`
	UptimeHours    float64 `json:"uptime_hours"`
	ProxyGBUsed    float64 `json:"proxy_gb_used"`
	ProxyGBBalance float64 `json:"proxy_gb_balance"`
}

type DeviceSession struct {
	DeviceID    string    `json:"device_id"`
	UserID      string    `json:"user_id"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
	UptimeHours float64   `json:"uptime_hours"`
}

func New(dbPath string) (*CreditService, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	cs := &CreditService{db: db}
	if err := cs.migrate(); err != nil {
		return nil, err
	}
	return cs, nil
}

func (cs *CreditService) migrate() error {
	_, err := cs.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			user_id TEXT PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			credits REAL DEFAULT 0,
			total_earned REAL DEFAULT 0,
			total_spent REAL DEFAULT 0,
			proxy_gb_used REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS device_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			connected_at DATETIME NOT NULL,
			disconnected_at DATETIME,
			uptime_hours REAL DEFAULT 0,
			credits_earned REAL DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(user_id)
		);

		CREATE TABLE IF NOT EXISTS credit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			amount REAL NOT NULL,
			reason TEXT NOT NULL,
			device_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(user_id)
		);

		CREATE TABLE IF NOT EXISTS proxy_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			bytes_used INTEGER NOT NULL,
			credits_deducted REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(user_id)
		);

		CREATE INDEX IF NOT EXISTS idx_sessions_user ON device_sessions(user_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_node ON device_sessions(node_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_active ON device_sessions(disconnected_at) WHERE disconnected_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_credit_log_user ON credit_log(user_id);
		CREATE INDEX IF NOT EXISTS idx_proxy_usage_user ON proxy_usage(user_id);
	`)
	return err
}

// ─── User Management ───────────────────────────────────────────────────────────

func (cs *CreditService) CreateUser(userID, token string) error {
	_, err := cs.db.Exec(
		"INSERT OR IGNORE INTO users (user_id, token) VALUES (?, ?)",
		userID, token,
	)
	return err
}

func (cs *CreditService) GetBalance(userID string) (*UserBalance, error) {
	var b UserBalance
	err := cs.db.QueryRow(
		"SELECT user_id, credits, total_earned, total_spent, proxy_gb_used FROM users WHERE user_id = ?",
		userID,
	).Scan(&b.UserID, &b.Credits, &b.TotalEarned, &b.TotalSpent, &b.ProxyGBUsed)
	if err != nil {
		return nil, err
	}

	// Active devices
	cs.db.QueryRow(
		"SELECT COUNT(*) FROM device_sessions WHERE user_id = ? AND disconnected_at IS NULL",
		userID,
	).Scan(&b.ActiveDevices)

	// Total uptime
	cs.db.QueryRow(
		"SELECT COALESCE(SUM(uptime_hours), 0) FROM device_sessions WHERE user_id = ?",
		userID,
	).Scan(&b.UptimeHours)

	b.ProxyGBBalance = b.Credits * DefaultGBPerCredit
	return &b, nil
}

// ─── Device Sessions ───────────────────────────────────────────────────────────

func (cs *CreditService) DeviceConnected(nodeID, userID string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	_, err := cs.db.Exec(
		`INSERT INTO device_sessions (device_id, user_id, node_id, connected_at)
		 VALUES (?, ?, ?, ?)`,
		nodeID, userID, nodeID, time.Now().UTC(),
	)
	return err
}

func (cs *CreditService) DeviceDisconnected(nodeID string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var sessionID int64
	var connectedAt time.Time
	var userID string
	err := cs.db.QueryRow(
		`SELECT id, user_id, connected_at FROM device_sessions 
		 WHERE node_id = ? AND disconnected_at IS NULL 
		 ORDER BY connected_at DESC LIMIT 1`,
		nodeID,
	).Scan(&sessionID, &userID, &connectedAt)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	hours := now.Sub(connectedAt).Hours()
	credits := cs.calculateCredits(userID, hours)

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Close session
	tx.Exec(
		"UPDATE device_sessions SET disconnected_at = ?, uptime_hours = ?, credits_earned = ? WHERE id = ?",
		now, hours, credits, sessionID,
	)

	// Add credits
	tx.Exec(
		"UPDATE users SET credits = credits + ?, total_earned = total_earned + ? WHERE user_id = ?",
		credits, credits, userID,
	)

	// Log
	tx.Exec(
		"INSERT INTO credit_log (user_id, amount, reason, device_id) VALUES (?, ?, ?, ?)",
		userID, credits, fmt.Sprintf("uptime %.1fh", hours), nodeID,
	)

	return tx.Commit()
}

func (cs *CreditService) calculateCredits(userID string, hours float64) float64 {
	credits := hours * CreditsPerHour

	// Uptime streak multiplier
	var totalUptime float64
	cs.db.QueryRow(
		`SELECT COALESCE(SUM(uptime_hours), 0) FROM device_sessions 
		 WHERE user_id = ? AND disconnected_at IS NOT NULL`,
		userID,
	).Scan(&totalUptime)

	if totalUptime+hours >= 72 {
		credits *= UptimeMultiplier72
	} else if totalUptime+hours >= 24 {
		credits *= UptimeMultiplier24
	}

	// Multi-device bonus
	var activeDevices int
	cs.db.QueryRow(
		"SELECT COUNT(*) FROM device_sessions WHERE user_id = ? AND disconnected_at IS NULL",
		userID,
	).Scan(&activeDevices)

	if activeDevices > 1 {
		credits *= 1.0 + float64(activeDevices-1)*MultiDeviceBonus
	}

	return credits
}

// ─── Proxy Usage (spend credits) ───────────────────────────────────────────────

func (cs *CreditService) DeductForProxy(userID string, bytesUsed int64) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	gbUsed := float64(bytesUsed) / (1024 * 1024 * 1024)
	creditsNeeded := gbUsed / DefaultGBPerCredit

	var currentCredits float64
	err := cs.db.QueryRow("SELECT credits FROM users WHERE user_id = ?", userID).Scan(&currentCredits)
	if err != nil {
		return fmt.Errorf("user not found: %s", userID)
	}
	if currentCredits < creditsNeeded {
		return fmt.Errorf("insufficient credits: have %.2f, need %.2f", currentCredits, creditsNeeded)
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(
		"UPDATE users SET credits = credits - ?, total_spent = total_spent + ?, proxy_gb_used = proxy_gb_used + ? WHERE user_id = ?",
		creditsNeeded, creditsNeeded, gbUsed, userID,
	)
	tx.Exec(
		"INSERT INTO proxy_usage (user_id, bytes_used, credits_deducted) VALUES (?, ?, ?)",
		userID, bytesUsed, creditsNeeded,
	)
	tx.Exec(
		"INSERT INTO credit_log (user_id, amount, reason) VALUES (?, ?, ?)",
		userID, -creditsNeeded, fmt.Sprintf("proxy %.4f GB", gbUsed),
	)

	return tx.Commit()
}

// ─── Periodic Credit Ticker (call every hour for active sessions) ──────────────

func (cs *CreditService) TickActiveCredits() {
	rows, err := cs.db.Query(
		`SELECT id, node_id, user_id, connected_at FROM device_sessions WHERE disconnected_at IS NULL`,
	)
	if err != nil {
		log.Printf("[CREDITS] tick error: %v", err)
		return
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var id int64
		var nodeID, userID string
		var connectedAt time.Time
		rows.Scan(&id, &nodeID, &userID, &connectedAt)

		hours := now.Sub(connectedAt).Hours()
		credits := cs.calculateCredits(userID, hours)

		// Update running totals (don't close session)
		cs.db.Exec(
			"UPDATE device_sessions SET uptime_hours = ?, credits_earned = ? WHERE id = ?",
			hours, credits, id,
		)
	}
}

func (cs *CreditService) Close() {
	cs.db.Close()
}
