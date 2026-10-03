package store

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func ConnectPostre() {
	var err error

	DB_URL := os.Getenv("DB_URL")

	if len(DB_URL) == 0 {
		log.Fatal("[ERROR] DB_URL env is missing")
	}

	DB, err = pgxpool.New(
		context.Background(),
		DB_URL,
	)

	if err != nil {
		log.Fatal("[ERROR] Connect database error: ", err)
	}
}

func CreateTables() {
	_, err := DB.Exec(
		context.Background(),
		`
		CREATE TABLE IF NOT EXISTS users (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		`,
	)

	if err != nil {
		log.Fatal("[ERROR] Create tables error: ", err)
	}

	_, err = DB.Exec(
		context.Background(),
		`
		CREATE TABLE IF NOT EXISTS refresh_tokens (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL
		);
		`,
	)

	if err != nil {
		log.Fatal("[ERROR] Create tables error: ", err)
	}

	_, err = DB.Exec(
		context.Background(),
		`
		CREATE TABLE IF NOT EXISTS collections (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			collection_name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (user_id, collection_name)
		);
		`,
	)

	if err != nil {
		log.Fatal("[ERROR] Create tables error: ", err)
	}
}
