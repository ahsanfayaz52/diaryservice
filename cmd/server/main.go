package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ahsanfayaz52/diaryservice/internal/auth"
	"github.com/ahsanfayaz52/diaryservice/internal/config"
	"github.com/ahsanfayaz52/diaryservice/internal/db"
	"github.com/ahsanfayaz52/diaryservice/internal/handlers"
)

func main() {
	cfg := config.LoadConfig()

	dbConn := db.InitDB(cfg.DatabasePath)
	defer dbConn.Close()

	jwtService := auth.NewJWTService(cfg.JWTSecret)

	r := mux.NewRouter()

	r.HandleFunc("/register", handlers.RegisterHandler(dbConn)).Methods("GET", "POST")
	r.HandleFunc("/login", handlers.LoginHandler(dbConn, jwtService)).Methods("GET", "POST")
	r.HandleFunc("/logout", handlers.LogoutHandler()).Methods("GET")
	r.HandleFunc("/ai/process", handlers.AIProcessHandler).Methods("POST")

	// Authenticated routes
	s := r.PathPrefix("/").Subrouter()
	s.Use(auth.JWTMiddleware(jwtService))

	s.HandleFunc("/dashboard", handlers.DashboardHandler(dbConn)).Methods("GET")
	s.HandleFunc("/notes/new", handlers.NewNoteHandler(dbConn)).Methods("GET", "POST")
	s.HandleFunc("/notes/edit/{id}", handlers.EditNoteHandler(dbConn)).Methods("GET", "POST")
	s.HandleFunc("/notes/delete/{id}", handlers.DeleteNoteHandler(dbConn)).Methods("POST")
	s.HandleFunc("/notes/view/{id}", handlers.ViewNoteHandler(dbConn)).Methods("GET")

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	log.Printf("Starting server on port %s...", cfg.Port)
	err := http.ListenAndServe(":"+cfg.Port, r)
	if err != nil {
		log.Fatal(err)
	}
}
