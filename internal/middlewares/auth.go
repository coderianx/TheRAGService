package middlewares

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"TheRAGService/internal/httpx"
	"TheRAGService/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsContextKey contextKey = "claims"

// Auth, Authorization header'ındaki "Bearer <token>" JWT'sini doğrular ve
// claim'leri request context'ine koyar. Token eksik, geçersiz veya süresi
// geçmişse 401 döner.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		tokenString, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tokenString == "" {
			httpx.WriteError(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}

		secret := os.Getenv("JWT_SECRET")

		if secret == "" {
			log.Println("[ERROR] JWT_SECRET env is missing")
			httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		claims := &models.Claims{}

		// WithValidMethods, yalnızca HS256 kabul edilmesini sağlar ve
		// algoritma karışıklığı (alg confusion) saldırılarını engeller.
		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (any, error) {
				return []byte(secret), nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		)

		if err != nil || !token.Valid {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext, Auth middleware'inin context'e koyduğu claim'leri döner.
// Handler içinde kimliği doğrulanmış kullanıcıya erişmek için kullanılır.
func ClaimsFromContext(ctx context.Context) (*models.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*models.Claims)

	return claims, ok
}
