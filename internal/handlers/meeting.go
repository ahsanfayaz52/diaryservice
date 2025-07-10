package handlers

import (
	"database/sql"
	"encoding/json"
	"github.com/ahsanfayaz52/diaryservice/internal/auth"
	"github.com/ahsanfayaz52/diaryservice/internal/stripe"
	"net/http"
)

func MeetingLimitsHandler(db *sql.DB, stripeSvc *stripe.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserIDFromContext(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get the latest limits from Stripe/database
		_, remainingSeconds, isSubscribed, err := stripeSvc.CheckUserLimits(db, userID)
		if err != nil {
			http.Error(w, "Failed to check limits", http.StatusInternalServerError)
			return
		}

		// Return as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isSubscribed":     isSubscribed,
			"remainingSeconds": remainingSeconds,
		})
	}
}
