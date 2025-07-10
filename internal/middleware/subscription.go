package middleware

import (
	"context"
	"database/sql"
	"github.com/ahsanfayaz52/diaryservice/internal/stripe"
	"net/http"
	_ "strconv"

	"github.com/ahsanfayaz52/diaryservice/internal/auth"
	"github.com/gorilla/mux"
)

func SubscriptionCheck(db *sql.DB, stripeSvc *stripe.Service) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := auth.GetUserIDFromContext(r.Context())
			if userID == 0 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			noteLimitExceeded, meetingLimitExceeded, err := stripeSvc.CheckUserLimits(db, userID)
			if err != nil {
				http.Error(w, "Failed to check subscription status", http.StatusInternalServerError)
				return
			}

			// Store limits in context
			ctx := context.WithValue(r.Context(), "subscription_limits", map[string]bool{
				"note_limit_exceeded":    noteLimitExceeded,
				"meeting_limit_exceeded": meetingLimitExceeded,
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
