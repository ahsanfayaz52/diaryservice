// db/db.go
package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(filepath string) *sql.DB {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	createUsersTable := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		stripe_customer_id TEXT,
		is_active BOOLEAN DEFAULT 0,
		subscription_id TEXT,
		plan_id TEXT,
		current_period_end DATETIME
	);`

	createNotesTable := `CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		title TEXT,
		content TEXT,
		tags TEXT,
		is_pinned BOOLEAN DEFAULT 0,
		is_starred BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	createUserLimitsTable := `CREATE TABLE IF NOT EXISTS user_limits (
		user_id INTEGER PRIMARY KEY,
		note_count INTEGER DEFAULT 0,
		meeting_seconds_used INTEGER DEFAULT 0,
		last_meeting_start DATETIME,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	_, err = db.Exec(createUsersTable)
	if err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}

	_, err = db.Exec(createNotesTable)
	if err != nil {
		log.Fatalf("Error creating notes table: %v", err)
	}

	_, err = db.Exec(createUserLimitsTable)
	if err != nil {
		log.Fatalf("Error creating user_limits table: %v", err)
	}

	return db
}
