package server

import (
	"fmt"
	"database/sql"
	"log"
	"time"
)

// User sent a request with invalid data.
type BadRequestError struct {
	Message string
}

func (e BadRequestError) Error() string {
	return e.Message
}

func ConnectDatabase(username, password string) *sql.DB {
	source := fmt.Sprintf("%s:%s@/ohmysmal?parseTime=true&loc=Local", username, password)
	db, err := sql.Open("mysql", source)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %s", err)
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping the database: %s", err)
	}

	log.Print("Successfully connected to the database")

	return db
}
