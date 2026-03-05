package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/infrastructure/auth"
)

type contextKey string

const UserIDKey contextKey = "userId"

func AuthMiddleware(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("token")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			userId, err := jwtService.ValidateToken(cookie.Value)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userId)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(r *http.Request) uuid.UUID {
	userId, _ := r.Context().Value(UserIDKey).(uuid.UUID)
	return userId
}
