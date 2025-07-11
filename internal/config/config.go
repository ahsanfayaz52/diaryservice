package config

import (
	"github.com/ahsanfayaz52/diaryservice/internal/stripe"
	"os"
	"strconv"
)

type Config struct {
	DBUser     string
	DBPassword string
	DBHost     string
	DBName     string

	JWTSecret string
	Port      string
	OpenAIKey string

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
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")

	// JWT configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	port := os.Getenv("PORT")

	// OpenAI configuration
	aiKey := os.Getenv("OPENAI_KEY")

	// Stripe configuration with test defaults
	stripeSecret := os.Getenv("STRIPE_SECRET_KEY")
	stripePubKey := os.Getenv("STRIPE_PUBLISHABLE_KEY")
	stripeWebhook := os.Getenv("STRIPE_WEBHOOK_SECRET")
	stripeMonthly := os.Getenv("STRIPE_MONTHLY_PLAN_ID")
	stripeAnnual := os.Getenv("STRIPE_ANNUAL_PLAN_ID")
	stripeMonthlyPrice := os.Getenv("STRIPE_MONTHLY_PRICE_ID")
	stripeAnnualPrice := os.Getenv("STRIPE_ANNUAL_PRICE_ID")
	// URLs with sensible local defaults
	stripeSuccess := os.Getenv("STRIPE_SUCCESS_URL")
	stripeCancel := os.Getenv("STRIPE_CANCEL_URL")
	encKey := os.Getenv("ENCRYPTION_KEY")

	freeNoteLimitStr := os.Getenv("FREE_NOTE_LIMIT")
	freeNoteLimit := 10 // default value
	if freeNoteLimitStr != "" {
		if val, err := strconv.Atoi(freeNoteLimitStr); err == nil {
			freeNoteLimit = val
		}
	}

	freeMeetingLimitStr := os.Getenv("FREE_MEETING_LIMIT")
	freeMeetingLimit := 60 // default value
	if freeMeetingLimitStr != "" {
		if val, err := strconv.Atoi(freeMeetingLimitStr); err == nil {
			freeMeetingLimit = val
		}
	}

	return &Config{
		DBUser:     dbUser,
		DBPassword: dbPassword,
		DBHost:     dbHost,
		DBName:     dbName,
		JWTSecret:  jwtSecret,
		Port:       port,
		OpenAIKey:  aiKey,

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
		FreeNoteLimit:   freeNoteLimit,    // Default free plan note limit
		FreeMeetingMins: freeMeetingLimit, // Default free plan meeting minutes
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
