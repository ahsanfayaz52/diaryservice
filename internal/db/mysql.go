// db/db.go
package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func InitDB(user, password, host, dbName string) *sql.DB {
	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", user, password, host, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL database: %v", err)
	}

	// Ping to verify connection
	if err := db.Ping(); err != nil {
		log.Fatalf("MySQL ping failed: %v", err)
	}

	createUsersTable := `CREATE TABLE IF NOT EXISTS users (
		id INT AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password VARCHAR(255) NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		stripe_customer_id VARCHAR(255),
		is_active BOOLEAN DEFAULT FALSE,
		subscription_id VARCHAR(255),
		plan_id VARCHAR(255),
		current_period_end DATETIME
	) ENGINE=InnoDB;`

	createNotesTable := `CREATE TABLE IF NOT EXISTS notes (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id INT NOT NULL,
		title TEXT,
		content TEXT,
		tags TEXT,
		is_pinned BOOLEAN DEFAULT FALSE,
		is_starred BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB;`

	createUserLimitsTable := `CREATE TABLE IF NOT EXISTS user_limits (
		user_id INT PRIMARY KEY,
		note_count INT DEFAULT 0,
		meeting_seconds_used INT DEFAULT 0,
		last_meeting_start DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB;`

	if _, err := db.Exec(createUsersTable); err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}
	if _, err := db.Exec(createNotesTable); err != nil {
		log.Fatalf("Error creating notes table: %v", err)
	}
	if _, err := db.Exec(createUserLimitsTable); err != nil {
		log.Fatalf("Error creating user_limits table: %v", err)
	}

	return db
}
