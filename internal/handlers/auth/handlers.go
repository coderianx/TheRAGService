package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"TheRAGService/internal/helpers"
	"TheRAGService/internal/httpx"
	"TheRAGService/internal/models"
	"TheRAGService/internal/store"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenTTL        = 30 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
)

func Register(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req models.RegisterReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "username, email and password are required")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("[ERROR] Hash password error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var res models.RegisterRes

	err = store.DB.QueryRow(
		r.Context(),
		`
		INSERT INTO users (username, email, password)
		VALUES ($1, $2, $3)
		RETURNING id, username, email
		`,
		req.Username,
		req.Email,
		string(hashed),
	).Scan(&res.ID, &res.Username, &res.Email)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httpx.WriteError(w, http.StatusConflict, "username or email already exists")
			return
		}

		log.Println("[ERROR] Insert user error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, res)
}

func Login(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req models.LoginReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Username == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	var (
		userID       int64
		username     string
		passwordHash string
	)

	err := store.DB.QueryRow(
		r.Context(),
		`
		SELECT id, username, password
		FROM users
		WHERE username = $1
		`,
		req.Username,
	).Scan(&userID, &username, &passwordHash)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		log.Println("[ERROR] Select user error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	secret := os.Getenv("JWT_SECRET")

	if secret == "" {
		log.Println("[ERROR] JWT_SECRET env is missing")
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	now := time.Now()
	expiresAt := now.Add(tokenTTL)

	claims := models.Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		log.Println("[ERROR] Sign token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	randomToken, err := helpers.GenerateRandomString(32)
	if err != nil {
		log.Println("[ERROR] Generate refresh token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	refreshTokenHash := sha256.Sum256([]byte(randomToken))

	_, err = store.DB.Exec(
		r.Context(),
		`
		INSERT INTO refresh_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		`,
		userID,
		hex.EncodeToString(refreshTokenHash[:]),
		now.Add(refreshTokenTTL),
	)

	if err != nil {
		log.Println("[ERROR] Insert refresh token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  token,
		"refresh_token": randomToken,
	})
}

func Refresh(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req models.RefreshReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.RefreshToken == "" {
		httpx.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	// Tokens are stored hashed, so look up by the SHA-256 hash.
	tokenHash := sha256.Sum256([]byte(req.RefreshToken))

	var (
		tokenID   int64
		userID    int64
		username  string
		expiresAt time.Time
	)

	err := store.DB.QueryRow(
		r.Context(),
		`
		SELECT rt.id, rt.user_id, rt.expires_at, u.username
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token = $1
		`,
		hex.EncodeToString(tokenHash[:]),
	).Scan(&tokenID, &userID, &expiresAt, &username)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}

		log.Println("[ERROR] Select refresh token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Expired tokens are rejected and removed from the database.
	if time.Now().After(expiresAt) {
		if _, err := store.DB.Exec(
			r.Context(),
			`DELETE FROM refresh_tokens WHERE id = $1`,
			tokenID,
		); err != nil {
			log.Println("[ERROR] Delete expired refresh token error: ", err)
		}

		httpx.WriteError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	secret := os.Getenv("JWT_SECRET")

	if secret == "" {
		log.Println("[ERROR] JWT_SECRET env is missing")
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	now := time.Now()
	accessExpiresAt := now.Add(tokenTTL)

	claims := models.Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		log.Println("[ERROR] Sign token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	randomToken, err := helpers.GenerateRandomString(32)
	if err != nil {
		log.Println("[ERROR] Generate refresh token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	newTokenHash := sha256.Sum256([]byte(randomToken))

	// Rotate atomically: delete the old token and store the new one
	// in a single transaction so no state is lost on failure.
	tx, err := store.DB.Begin(r.Context())
	if err != nil {
		log.Println("[ERROR] Begin transaction error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(
		r.Context(),
		`DELETE FROM refresh_tokens WHERE id = $1`,
		tokenID,
	); err != nil {
		log.Println("[ERROR] Delete refresh token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if _, err := tx.Exec(
		r.Context(),
		`
		INSERT INTO refresh_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		`,
		userID,
		hex.EncodeToString(newTokenHash[:]),
		now.Add(refreshTokenTTL),
	); err != nil {
		log.Println("[ERROR] Insert refresh token error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Println("[ERROR] Commit transaction error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": randomToken,
	})
}
