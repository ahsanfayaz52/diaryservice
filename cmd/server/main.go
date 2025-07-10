package main

import (
	"github.com/ahsanfayaz52/diaryservice/internal/encryption"
	"github.com/ahsanfayaz52/diaryservice/internal/middleware"
	"github.com/ahsanfayaz52/diaryservice/internal/stripe"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/ahsanfayaz52/diaryservice/internal/auth"
	"github.com/ahsanfayaz52/diaryservice/internal/config"
	"github.com/ahsanfayaz52/diaryservice/internal/db"
	"github.com/ahsanfayaz52/diaryservice/internal/handlers"
)

func main() {
	cfg := config.LoadConfig()

	dbConn := db.InitDB(cfg.DatabasePath)
	defer dbConn.Close()

	stripeSvc := stripe.NewService(cfg.StripeConfig())

	encryptionSvc, err := encryption.NewService(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize encryption service: %v", err)
	}

	jwtService := auth.NewJWTService(cfg.JWTSecret)

	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	// Handlers
	subscriptionHandler := handlers.NewSubscriptionHandler(dbConn, stripeSvc, cfg)

	r.HandleFunc("/register", handlers.RegisterHandler(dbConn)).Methods("GET", "POST")
	r.HandleFunc("/login", handlers.LoginHandler(dbConn, jwtService)).Methods("GET", "POST")
	r.HandleFunc("/logout", handlers.LogoutHandler()).Methods("GET")
	r.HandleFunc("/ai/process", handlers.AIProcessHandler).Methods("POST")
	r.HandleFunc("/ai/summarize-meeting", handlers.SummarizeMeetingHandler).Methods("POST")
	r.HandleFunc("/api/subscription/webhook", subscriptionHandler.WebhookHandler).Methods("POST")

	// In your main router setup (main.go or routes.go)
	r.HandleFunc("/privacy", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles(
			"templates/base.html",
			"templates/privacy.html",
		))

		data := map[string]interface{}{
			"CurrentPage": "privacy",
			"CurrentYear": time.Now().Year(),
		}

		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("/terms", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles(
			"templates/base.html",
			"templates/terms.html",
		))

		data := map[string]interface{}{
			"CurrentPage": "terms",
			"CurrentYear": time.Now().Year(),
		}

		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Authenticated routes
	s := r.PathPrefix("/").Subrouter()
	s.Use(auth.JWTMiddleware(jwtService))
	s.Use(middleware.SubscriptionCheck(dbConn, stripeSvc))

	s.HandleFunc("/subscription", subscriptionHandler.SubscriptionPageHandler).Methods("GET")
	s.HandleFunc("/api/meeting/start", subscriptionHandler.MeetingStart).Methods("POST")
	s.HandleFunc("/api/meeting/end", subscriptionHandler.MeetingEnd).Methods("POST")
	s.HandleFunc("/api/subscription/checkout", subscriptionHandler.CreateCheckoutSession).Methods("POST")
	s.HandleFunc("/api/subscription/status", subscriptionHandler.GetSubscriptionStatus).Methods("GET")
	s.HandleFunc("/api/subscription/cancel", subscriptionHandler.CancelSubscription).Methods("POST")

	s.HandleFunc("/dashboard", handlers.DashboardHandler(dbConn, encryptionSvc)).Methods("GET")
	s.HandleFunc("/notes/new", handlers.NewNoteHandler(dbConn, stripeSvc, encryptionSvc)).Methods("GET", "POST")
	s.HandleFunc("/notes/edit/{id}", handlers.EditNoteHandler(dbConn, stripeSvc, encryptionSvc)).Methods("GET", "POST")
	s.HandleFunc("/notes/delete/{id}", handlers.DeleteNoteHandler(dbConn)).Methods("POST")
	s.HandleFunc("/notes/view/{id}", handlers.ViewNoteHandler(dbConn, encryptionSvc)).Methods("GET")

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	log.Printf("Starting server on port %s...", cfg.Port)
	err = http.ListenAndServe(":"+cfg.Port, r)
	if err != nil {
		log.Fatal(err)
	}
}
