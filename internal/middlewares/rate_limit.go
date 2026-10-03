package middlewares

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"TheRAGService/internal/httpx"
	"TheRAGService/internal/store"

	"github.com/redis/go-redis/v9"
)

// rateLimitScript, sayaç artırma ve ilk istekte pencere (TTL) atama işlemini
// tek adımda atomik olarak yapar; aksi halde INCR ile EXPIRE arasında oluşan
// yarış durumunda anahtar TTL'siz kalıp kalıcı olarak bloklayabilir.
var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// RateLimitByIP, istemci IP'si başına sabit pencereli (fixed window) istek
// limiti uygular. Pencere içinde limit aşılırsa 429 döner.
func RateLimitByIP(limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := "rate_limit:" + ip

			count, err := rateLimitScript.Run(
				r.Context(),
				store.RedisClient,
				[]string{key},
				window.Milliseconds(),
			).Int64()

			if err != nil {
				// Redis erişilemezse istekleri engellemeyip trafiği geçiriyoruz
				// (fail-open); aksi halde Redis kesintisi tüm API'yi durdurur.
				log.Println("[ERROR] Rate limit error: ", err)
				next.ServeHTTP(w, r)
				return
			}

			remaining := limit - count
			if remaining < 0 {
				remaining = 0
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

			if count > limit {
				w.Header().Set("Retry-After", strconv.FormatInt(int64(window.Seconds()), 10))
				httpx.WriteError(w, http.StatusTooManyRequests, "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// clientIP, proxy arkasındaki gerçek istemci IP'sini çözmeye çalışır.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// İlk değer, istemciye en yakın (orijinal) adrestir.
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
