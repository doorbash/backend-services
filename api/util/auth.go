package util

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/doorbash/backend-services-api/api/domain"
	"github.com/go-redis/redis/v8"
)

func OAuth2Middleware(authCache domain.AuthCache, admin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.Split(header, " ")

		if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
			log.Println("bad Authorization header")
			WriteUnauthorized(w)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		token := parts[1]
		email, err := authCache.GetEmailByToken(ctx, token)

		if err != nil {
			log.Println(err)
			if err == redis.Nil {
				WriteUnauthorized(w)
			} else {
				WriteInternalServerError(w)
			}
			return
		}

		r.Header.Set("email", email)
		if email == admin {
			r.Header.Set("role", "admin")
		} else {
			r.Header.Set("role", "member")
		}
		r.Header.Set("token", token)
		next.ServeHTTP(w, r)
		r.Header.Del("email")
		r.Header.Del("role")
		r.Header.Del("token")
	})
}
