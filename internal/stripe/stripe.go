// internal/stripe/stripe.go
package stripe

import (
	"database/sql"
	"fmt"
	"github.com/stripe/stripe-go/v76"
	"time"

	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/webhook"
)

type Config struct {
	SecretKey       string
	WebhookSecret   string
	MonthlyPlanID   string
	AnnualPlanID    string
	FreeNoteLimit   int
	FreeMeetingMins int
}

type Service struct {
	Config Config
}

func NewService(cfg Config) *Service {
	stripe.Key = cfg.SecretKey
	return &Service{Config: cfg}
}

func (s *Service) CreateCustomer(email string) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}
	c, err := customer.New(params)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func (s *Service) CreateSubscription(customerID, planID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Plan: stripe.String(planID),
			},
		},
	}
	return subscription.New(params)
}

func (s *Service) HandleWebhook(payload []byte, sigHeader string) (stripe.Event, error) {
	return webhook.ConstructEventWithOptions(payload, sigHeader, s.Config.WebhookSecret,
		stripe.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
}

func (s *Service) GetSubscription(subID string) (*stripe.Subscription, error) {
	return subscription.Get(subID, nil)
}

func (s *Service) CheckUserLimits(db *sql.DB, userID int) (bool, bool, error) {
	var noteCount, meetingSeconds int
	var isActive bool
	var subEnd sql.NullTime // Changed to sql.NullTime

	// Get user subscription status
	err := db.QueryRow("SELECT is_active, current_period_end FROM users WHERE id = ?", userID).Scan(&isActive, &subEnd)
	if err != nil {
		if err == sql.ErrNoRows {
			// User not found - handle appropriately
			return true, true, nil // Or return an error if that's more appropriate
		}
		return false, false, fmt.Errorf("error getting subscription status: %w", err)
	}

	// Get user limits
	err = db.QueryRow("SELECT COALESCE(note_count, 0), COALESCE(meeting_seconds_used, 0) FROM user_limits WHERE user_id = ?", userID).Scan(&noteCount, &meetingSeconds)
	if err != nil && err != sql.ErrNoRows {
		return false, false, fmt.Errorf("error getting user limits: %w", err)
	}

	// Check if subscription is active
	subscriptionActive := isActive && subEnd.Valid && subEnd.Time.After(time.Now())

	// Check note limit (10 for free users)
	noteLimitExceeded := !subscriptionActive && noteCount >= s.Config.FreeNoteLimit

	// Check meeting limit (5 minutes for free users)
	meetingLimitExceeded := !subscriptionActive && meetingSeconds >= s.Config.FreeMeetingMins*60

	return noteLimitExceeded, meetingLimitExceeded, nil
}
