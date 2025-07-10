package config

import (
	"github.com/ahsanfayaz52/diaryservice/internal/stripe"
	"os"
)

type Config struct {
	DatabasePath string
	JWTSecret    string
	Port         string
	OpenAIKey    string

	// Stripe Configuration
	StripeSecretKey      string
	StripePublishableKey string
	StripeWebhookSecret  string
	StripeMonthlyPlanID  string
	StripeAnnualPlanID   string
	StripeMonthlyPriceID string
	StripeAnnualPriceID  string
	StripeSuccessURL     string
	StripeCancelURL      string

	EncryptionKey string `yaml:"encryption_key"`

	// Business Logic Limits
	FreeNoteLimit   int
	FreeMeetingMins int
}

func LoadConfig() *Config {
	// Database configuration
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/go-diary.db"
	}

	// JWT configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "" // Always override in production!
	}

	// Server configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	// OpenAI configuration
	aiKey := os.Getenv("OPENAI_KEY")
	if aiKey == "" {
		aiKey = "" // Never commit real keys to code!
	}

	// Stripe configuration with test defaults
	stripeSecret := os.Getenv("STRIPE_SECRET_KEY")
	if stripeSecret == "" {
		stripeSecret = "" // Test key only - never use in production
	}

	stripePubKey := os.Getenv("STRIPE_PUBLISHABLE_KEY")
	if stripePubKey == "" {
		stripePubKey = "" // Test key only
	}

	stripeWebhook := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if stripeWebhook == "" {
		stripeWebhook = "" // Test secret only
	}

	stripeMonthly := os.Getenv("STRIPE_MONTHLY_PLAN_ID")
	if stripeMonthly == "" {
		stripeMonthly = "" // Test plan ID
	}

	stripeAnnual := os.Getenv("STRIPE_ANNUAL_PLAN_ID")
	if stripeAnnual == "" {
		stripeAnnual = "" // Test plan ID
	}

	stripeMonthlyPrice := os.Getenv("STRIPE_MONTHLY_PRICE_ID")
	if stripeMonthlyPrice == "" {
		stripeMonthlyPrice = "" // Test plan ID
	}

	stripeAnnualPrice := os.Getenv("STRIPE_ANNUAL_PRICE_ID")
	if stripeAnnualPrice == "" {
		stripeAnnualPrice = "" // Test plan ID
	}

	// URLs with sensible local defaults
	stripeSuccess := os.Getenv("STRIPE_SUCCESS_URL")
	if stripeSuccess == "" {
		stripeSuccess = "http://127.0.0.1:8086/dashboard?payment=success"
	}

	stripeCancel := os.Getenv("STRIPE_CANCEL_URL")
	if stripeCancel == "" {
		stripeCancel = "http://127.0.0.1:8086/subscription?payment=cancel"
	}

	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		encKey = ""
	}

	return &Config{
		DatabasePath: dbPath,
		JWTSecret:    jwtSecret,
		Port:         port,
		OpenAIKey:    aiKey,

		// Stripe Config
		StripeSecretKey:      stripeSecret,
		StripePublishableKey: stripePubKey,
		StripeWebhookSecret:  stripeWebhook,
		StripeMonthlyPlanID:  stripeMonthly,
		StripeAnnualPlanID:   stripeAnnual,
		StripeMonthlyPriceID: stripeMonthlyPrice,
		StripeAnnualPriceID:  stripeAnnualPrice,
		StripeSuccessURL:     stripeSuccess,
		StripeCancelURL:      stripeCancel,
		EncryptionKey:        encKey,

		// Business Limits
		FreeNoteLimit:   10, // Default free plan note limit
		FreeMeetingMins: 60, // Default free plan meeting minutes
	}
}

// StripeConfig returns a stripe-specific configuration struct
func (c *Config) StripeConfig() stripe.Config {
	return stripe.Config{
		SecretKey:       c.StripeSecretKey,
		WebhookSecret:   c.StripeWebhookSecret,
		MonthlyPlanID:   c.StripeMonthlyPlanID,
		AnnualPlanID:    c.StripeAnnualPlanID,
		MonthlyPriceID:  c.StripeMonthlyPriceID,
		AnnualPriceID:   c.StripeAnnualPriceID,
		FreeNoteLimit:   c.FreeNoteLimit,
		FreeMeetingMins: c.FreeMeetingMins,
	}
}
