package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/go-redis/redis/v8"
)

type AuthUserValue struct {
	ID      int
	Email   string
	IsAdmin bool
	Token   string
}

func OAuth2Middleware(authCache domain.AuthCache, admin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.Split(header, " ")

		if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
			log.Println("bad Authorization header")
			util.WriteUnauthorized(w)
			return
		}

		ctx, cancel := util.GetContextWithTimeout(context.Background())
		defer cancel()
		token := parts[1]
		email, id, err := authCache.GetUserByToken(ctx, token)

		if err != nil {
			log.Println(err)
			if err == redis.Nil {
				util.WriteUnauthorized(w)
			} else {
				util.WriteInternalServerError(w)
			}
			return
		}

		ctx = context.WithValue(r.Context(), "user", AuthUserValue{
			ID:      id,
			Email:   email,
			IsAdmin: email == admin,
			Token:   token,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
