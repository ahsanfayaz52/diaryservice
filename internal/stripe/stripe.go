// internal/stripe/stripe.go
package stripe

import (
	"database/sql"
	"fmt"
	"github.com/stripe/stripe-go/v76"
	"math"
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
	MonthlyPriceID  string
	AnnualPriceID   string
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
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
}

func (s *Service) GetSubscription(subID string) (*stripe.Subscription, error) {
	return subscription.Get(subID, nil)
}

func (s *Service) CheckUserLimits(db *sql.DB, userID int) (noteLimitExceeded bool, remainingSeconds int, isSubscribed bool, err error) {
	// Initialize default values
	noteLimitExceeded = false
	isSubscribed = false
	remainingSeconds = 0

	// Get user subscription status
	var subEnd sql.NullTime
	err = db.QueryRow(`
        SELECT is_active, current_period_end 
        FROM users 
        WHERE id = ?`, userID).Scan(&isSubscribed, &subEnd)

	if err != nil {
		if err == sql.ErrNoRows {
			return true, 0, false, fmt.Errorf("user not found")
		}
		return false, 0, false, fmt.Errorf("error getting subscription status: %w", err)
	}

	// Check if subscription is active
	subscriptionActive := isSubscribed && subEnd.Valid && subEnd.Time.After(time.Now())

	// Get user limits
	var noteCount, meetingSecondsUsed int
	err = db.QueryRow(`
        SELECT COALESCE(note_count, 0), COALESCE(meeting_seconds_used, 0) 
        FROM user_limits 
        WHERE user_id = ?`, userID).Scan(&noteCount, &meetingSecondsUsed)

	if err != nil && err != sql.ErrNoRows {
		return false, 0, false, fmt.Errorf("error getting user limits: %w", err)
	}

	// Calculate remaining meeting seconds
	if subscriptionActive {
		remainingSeconds = math.MaxInt32 // Unlimited for subscribed users
	} else {
		remainingSeconds = s.Config.FreeMeetingMins*60 - meetingSecondsUsed
		if remainingSeconds < 0 {
			remainingSeconds = 0
		}
	}

	// Check note limit
	noteLimitExceeded = !subscriptionActive && noteCount >= s.Config.FreeNoteLimit

	return noteLimitExceeded, remainingSeconds, subscriptionActive, nil
}

func (s *Service) CancelSubscription(id string) error {
	// Cancel the subscription immediately
	_, err := subscription.Cancel(id, nil)
	if err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}
	return nil
}
