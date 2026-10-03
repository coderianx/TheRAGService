package main

import (
	authhandlers "TheRAGService/internal/handlers/auth"
	"TheRAGService/internal/middlewares"
	"TheRAGService/internal/store"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("[ERROR] Load .env file error", err)
	}

	r := chi.NewRouter()

	// Postresql database
	store.ConnectPostre()
	store.CreateTables()
	// Redis database
	store.ConnectRedis()

	r.Use(middleware.Logger)

	r.Route("/api/v1", func(r chi.Router) {
		r.With(
			middlewares.RateLimitByIP(
				3,
				time.Minute,
			),
		).Post("/register", authhandlers.Register)

		r.With(
			middlewares.RateLimitByIP(
				5,
				time.Minute,
			),
		).Post("/login", authhandlers.Login)

		r.With(
			middlewares.RateLimitByIP(
				5,
				time.Minute,
			),
		).Post("/refresh", authhandlers.Refresh)
	})

	// Start web server
	fmt.Println("Golang Server starting on :8080")
	http.ListenAndServe(":8080", r)

}
