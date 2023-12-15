package models

import (
	"database/sql"
	"time"
)

type Website struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	DateAdded  string `json:"data_added"`
	Uptime     string `json:"uptime"`
	Interval   int    `json:"interval"`
}

// openDatabase opens the sqlite database from a file.
func OpenDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./database.db")
	if err != nil {
		return nil, err
	}

	// Verify database is accessible.
	if err = db.Ping(); err != nil {
		return nil, err
	}
	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS websites (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			url TEXT NOT NULL UNIQUE,
			status_code INTEGER,
			date_added DATETIME CURRENT_TIMESTAMP,
			uptime TEXT,
			interval INTEGER
		);
		CREATE INDEX IF NOT EXISTS idx_websites_date_added ON websites(date_added);
	`)
	if err != nil {
		return &sql.DB{}, nil
	}
	// Limit maximum open connections
	db.SetMaxOpenConns(10)
	// Limit number of idle connections
	db.SetMaxIdleConns(5)
	// Set idle connection timeout
	db.SetConnMaxIdleTime(1 * time.Second)
	// Set connection lifetime
	db.SetConnMaxLifetime(30 * time.Second)

	return db, nil
}
