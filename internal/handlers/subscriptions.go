// handlers/subscriptions.go
package handlers

import (
	"database/sql"
	"encoding/json"
	_ "fmt"
	"github.com/ahsanfayaz52/diaryservice/internal/config"
	"github.com/ahsanfayaz52/diaryservice/internal/stripe"
	"github.com/stripe/stripe-go/v76/product"
	"github.com/stripe/stripe-go/v76/webhook"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ahsanfayaz52/diaryservice/internal/auth"
	_ "github.com/gorilla/mux"
	stripeapi "github.com/stripe/stripe-go/v76"
	checkout "github.com/stripe/stripe-go/v76/checkout/session"
)

type SubscriptionHandler struct {
	db        *sql.DB
	stripeSvc *stripe.Service
	cfg       *config.Config
}

func NewSubscriptionHandler(db *sql.DB, stripeSvc *stripe.Service, cfg *config.Config) *SubscriptionHandler {
	return &SubscriptionHandler{db: db, stripeSvc: stripeSvc, cfg: cfg}
}

func (h *SubscriptionHandler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ProductType string `json:"product_type"` // "monthly" or "annual"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get user email
	var email string
	err := h.db.QueryRow("SELECT email FROM users WHERE id = ?", userID).Scan(&email)
	if err != nil {
		http.Error(w, "Failed to get user email", http.StatusInternalServerError)
		return
	}

	// Get or create Stripe customer
	var customerID string
	err = h.db.QueryRow("SELECT stripe_customer_id FROM users WHERE id = ?", userID).Scan(&customerID)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Failed to get customer ID", http.StatusInternalServerError)
		return
	}

	if customerID == "" {
		customerID, err = h.stripeSvc.CreateCustomer(email)
		if err != nil {
			http.Error(w, "Failed to create customer", http.StatusInternalServerError)
			return
		}

		_, err = h.db.Exec("UPDATE users SET stripe_customer_id = ? WHERE id = ?", customerID, userID)
		if err != nil {
			http.Error(w, "Failed to save customer ID", http.StatusInternalServerError)
			return
		}
	}

	// Get the default price for the selected product
	var priceID string
	switch req.ProductType {
	case "premium":
		priceID = h.getDefaultPriceID(h.cfg.StripeMonthlyPlanID)
	case "pro":
		priceID = h.getDefaultPriceID(h.cfg.StripeAnnualPlanID)
	default:
		http.Error(w, "Invalid product type", http.StatusBadRequest)
		return
	}

	if priceID == "" {
		http.Error(w, "Failed to get product price", http.StatusInternalServerError)
		return
	}

	// Create checkout session
	params := &stripeapi.CheckoutSessionParams{
		Customer: stripeapi.String(customerID),
		PaymentMethodTypes: stripeapi.StringSlice([]string{
			"card",
		}),
		LineItems: []*stripeapi.CheckoutSessionLineItemParams{
			{
				Price:    stripeapi.String(priceID),
				Quantity: stripeapi.Int64(1),
			},
		},
		Mode:       stripeapi.String(string(stripeapi.CheckoutSessionModeSubscription)),
		SuccessURL: stripeapi.String(h.cfg.StripeSuccessURL),
		CancelURL:  stripeapi.String(h.cfg.StripeCancelURL),
	}

	sess, err := checkout.New(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(struct {
		SessionID string `json:"sessionId"`
	}{
		SessionID: sess.ID,
	})
}

func (h *SubscriptionHandler) getDefaultPriceID(productID string) string {
	params := &stripeapi.ProductParams{}
	params.AddExpand("default_price")
	p, _ := product.Get(productID, params)

	if p != nil && p.DefaultPrice != nil {
		return p.DefaultPrice.ID
	}

	return ""
}

func (h *SubscriptionHandler) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err) // Add this
		http.Error(w, "Error reading request body", http.StatusServiceUnavailable)
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		payload,
		r.Header.Get("Stripe-Signature"),
		h.stripeSvc.Config.WebhookSecret,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
	if err != nil {
		log.Printf("Webhook verification failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var session stripeapi.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			http.Error(w, "Error parsing webhook JSON", http.StatusBadRequest)
			return
		}

		// Get subscription
		sub, err := h.stripeSvc.GetSubscription(session.Subscription.ID)
		if err != nil {
			http.Error(w, "Error getting subscription", http.StatusBadRequest)
			return
		}

		log.Printf("sub event: sub_is: %v, plan_id: %v, current_id %v, cust_id: %v", sub.ID, sub.Items.Data[0].Plan.ID, time.Unix(sub.CurrentPeriodEnd, 0), session.Customer.ID)
		// Update user in database
		_, err = h.db.Exec(`UPDATE users SET 
			is_active = 1,
			subscription_id = ?,
			plan_id = ?,
			current_period_end = ?
			WHERE stripe_customer_id = ?`,
			sub.ID,
			sub.Items.Data[0].Plan.ID,
			time.Unix(sub.CurrentPeriodEnd, 0),
			session.Customer.ID)
		if err != nil {
			log.Printf("err: %v", err)
			http.Error(w, "Error updating user subscription", http.StatusInternalServerError)
			return
		}

	case "invoice.payment_succeeded":
		var invoice stripeapi.Invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			http.Error(w, "Error parsing webhook JSON", http.StatusBadRequest)
			return
		}

		if invoice.Subscription != nil {
			// Update subscription period
			sub, err := h.stripeSvc.GetSubscription(invoice.Subscription.ID)
			if err != nil {
				http.Error(w, "Error getting subscription", http.StatusBadRequest)
				return
			}

			_, err = h.db.Exec(`UPDATE users SET 
				current_period_end = ?
				WHERE subscription_id = ?`,
				time.Unix(sub.CurrentPeriodEnd, 0),
				sub.ID)
			if err != nil {
				http.Error(w, "Error updating subscription period", http.StatusInternalServerError)
				return
			}
		}

	case "customer.subscription.deleted", "customer.subscription.updated":
		var sub stripeapi.Subscription
		err := json.Unmarshal(event.Data.Raw, &sub)
		if err != nil {
			http.Error(w, "Error parsing webhook JSON", http.StatusBadRequest)
			return
		}

		// Update user subscription status
		isActive := event.Type != "customer.subscription.deleted"
		_, err = h.db.Exec(`UPDATE users SET 
			is_active = ?,
			current_period_end = ?
			WHERE subscription_id = ?`,
			isActive,
			time.Unix(sub.CurrentPeriodEnd, 0),
			sub.ID)
		if err != nil {
			http.Error(w, "Error updating subscription status", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *SubscriptionHandler) GetSubscriptionStatus(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var status struct {
		IsActive         bool      `json:"is_active"`
		PlanID           string    `json:"plan_id"`
		CurrentPeriodEnd time.Time `json:"current_period_end"`
		NoteCount        int       `json:"note_count"`
		MeetingSeconds   int       `json:"meeting_seconds"`
		NoteLimit        int       `json:"note_limit"`
		MeetingLimit     int       `json:"meeting_limit"`
	}

	err := h.db.QueryRow(`SELECT u.is_active, u.plan_id, u.current_period_end, 
		COALESCE(ul.note_count, 0), COALESCE(ul.meeting_seconds_used, 0)
		FROM users u
		LEFT JOIN user_limits ul ON u.id = ul.user_id
		WHERE u.id = ?`, userID).Scan(
		&status.IsActive,
		&status.PlanID,
		&status.CurrentPeriodEnd,
		&status.NoteCount,
		&status.MeetingSeconds,
	)
	if err != nil {
		http.Error(w, "Failed to get subscription status", http.StatusInternalServerError)
		return
	}

	status.NoteLimit = h.stripeSvc.Config.FreeNoteLimit
	status.MeetingLimit = h.stripeSvc.Config.FreeMeetingMins * 60

	json.NewEncoder(w).Encode(status)
}

func (h *SubscriptionHandler) MeetingStart(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	_, err := h.db.Exec(`INSERT INTO user_limits (user_id, last_meeting_start) 
		VALUES (?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET last_meeting_start = CURRENT_TIMESTAMP`,
		userID)
	if err != nil {
		http.Error(w, "Failed to record meeting start", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *SubscriptionHandler) MeetingEnd(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var startTime time.Time
	err := h.db.QueryRow("SELECT last_meeting_start FROM user_limits WHERE user_id = ?", userID).Scan(&startTime)
	if err != nil {
		http.Error(w, "Failed to get meeting start time", http.StatusInternalServerError)
		return
	}

	duration := int(time.Since(startTime).Seconds())

	_, err = h.db.Exec(`UPDATE user_limits 
		SET meeting_seconds_used = meeting_seconds_used + ?
		WHERE user_id = ?`,
		duration, userID)
	if err != nil {
		http.Error(w, "Failed to update meeting duration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *SubscriptionHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user's subscription ID from database
	var subscriptionID string
	err := h.db.QueryRow("SELECT subscription_id FROM users WHERE id = ?", userID).Scan(&subscriptionID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "No subscription found", http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to get subscription", http.StatusInternalServerError)
		return
	}

	// Cancel subscription with Stripe
	err = h.stripeSvc.CancelSubscription(subscriptionID)
	if err != nil {
		http.Error(w, "Failed to cancel subscription", http.StatusInternalServerError)
		return
	}

	// Update user status in database
	_, err = h.db.Exec(`UPDATE users SET 
        is_active = 0,
        subscription_id = NULL,
        plan_id = NULL,
        current_period_end = NULL
        WHERE id = ?`, userID)
	if err != nil {
		http.Error(w, "Failed to update user status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *SubscriptionHandler) SubscriptionPageHandler(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user's current subscription status
	var status struct {
		IsActive       bool
		PlanID         sql.NullString
		NoteCount      int
		MeetingMinutes int
		CurrentEndTime sql.NullTime
	}

	err := h.db.QueryRow(`
    SELECT u.is_active, u.plan_id, 
           COALESCE(ul.note_count, 0), 
           COALESCE(ul.meeting_seconds_used, 0)/60,
           u.current_period_end
    FROM users u
    LEFT JOIN user_limits ul ON u.id = ul.user_id
    WHERE u.id = ?`, userID).Scan(
		&status.IsActive,
		&status.PlanID,
		&status.NoteCount,
		&status.MeetingMinutes,
		&status.CurrentEndTime,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		log.Printf("Error querying subscription status: %v", err)
		http.Error(w, "Error retrieving subscription status", http.StatusInternalServerError)
		return
	}

	// Get plan name - handle NULL PlanID
	planName := "Free"
	if status.PlanID.Valid {
		planName = getPlanName(status.PlanID.String, h.cfg)
	}

	tmpl := template.Must(template.ParseFiles(
		"templates/base.html",
		"templates/subscription.html",
	))

	tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
		"IsActive":               status.IsActive,
		"PlanName":               planName,
		"CurrentPeriodEnd":       status.CurrentEndTime.Time, // Will be zero time if NULL
		"NoteCount":              status.NoteCount,
		"MeetingMinutes":         status.MeetingMinutes,
		"StripePublishableKey":   h.cfg.StripePublishableKey,
		"StripeMonthlyProductID": h.cfg.StripeMonthlyPlanID,
		"StripeAnnualProductID":  h.cfg.StripeAnnualPlanID,
	})
}

func getPlanName(planID string, cfg *config.Config) string {
	switch planID {
	case cfg.StripeMonthlyPriceID:
		return "premium"
	case cfg.StripeAnnualPriceID:
		return "pro"
	default:
		return "Free"
	}
}
