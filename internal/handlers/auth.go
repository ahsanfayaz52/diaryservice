package handlers

import (
	"database/sql"
	"net/http"
	"text/template"
	"time"

	"github.com/ahsanfayaz52/diaryservice/internal/auth"
	"github.com/ahsanfayaz52/diaryservice/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("templates/register.html", "templates/base.html"))

		if r.Method == http.MethodGet {
			tmpl.ExecuteTemplate(w, "base.html", nil)
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		// hash password
		hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Error creating user", http.StatusInternalServerError)
			return
		}

		_, err = db.Exec("INSERT INTO users (email, password) VALUES (?, ?)", email, string(hashedPass))
		if err != nil {
			http.Error(w, "Email already registered", http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func LoginHandler(db *sql.DB, jwtService *auth.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("templates/login.html", "templates/base.html"))

		if r.Method == http.MethodGet {
			tmpl.ExecuteTemplate(w, "base.html", nil)
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		var user models.User
		row := db.QueryRow("SELECT id, password FROM users WHERE email=?", email)
		err := row.Scan(&user.ID, &user.Password)
		if err != nil {
			// Pass error message to template
			tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
				"Error": "Invalid email or password",
				"Email": email, // Preserve the email so user doesn't have to retype
			})
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
		if err != nil {
			// Pass error message to template
			tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
				"Error": "Invalid email or password",
				"Email": email, // Preserve the email so user doesn't have to retype
			})
			return
		}

		token, err := jwtService.GenerateToken(user.ID)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    token,
			HttpOnly: true,
			Path:     "/",
			Expires:  time.Now().Add(72 * time.Hour),
		})

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    "",
			HttpOnly: true,
			Path:     "/",
			MaxAge:   -1,
		})

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
