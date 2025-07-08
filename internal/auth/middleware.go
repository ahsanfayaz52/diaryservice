package auth

import (
	"context"
	"net/http"
)

type key int

const UserIDKey key = 0

func JWTMiddleware(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("token")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			tokenStr := cookie.Value
			userID, err := jwtService.ValidateToken(tokenStr)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserIDFromContext(ctx context.Context) int {
	userID, ok := ctx.Value(UserIDKey).(int)
	if !ok {
		return 0
	}
	return userID
}
