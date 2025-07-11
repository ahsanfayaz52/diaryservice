package handlers

import (
	"database/sql"
	"github.com/ahsanfayaz52/diaryservice/internal/auth"
	"github.com/ahsanfayaz52/diaryservice/internal/encryption"
	"github.com/ahsanfayaz52/diaryservice/internal/models"
	"github.com/ahsanfayaz52/diaryservice/internal/stripe"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func DashboardHandler(db *sql.DB, encryptionSvc *encryption.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var isAuthenticated bool
		userID := auth.GetUserIDFromContext(r.Context())
		if userID != 0 {
			isAuthenticated = true
		}
		query := r.URL.Query()

		search := query.Get("search")
		tags := query["tag"]
		filterPinned := query.Get("filter_pinned") == "true"
		filterStarred := query.Get("filter_starred") == "true"
		sortBy := query.Get("sort_by")
		page := 1
		if p := query.Get("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil {
				page = parsed
			}
		}
		pageSize := 9
		offset := (page - 1) * pageSize

		where := []string{"user_id = ?"}
		args := []interface{}{userID}

		if search != "" {
			where = append(where, "(title LIKE ? OR content LIKE ?)")
			args = append(args, "%"+search+"%", "%"+search+"%")
		}

		// Modified tag filtering to use OR condition
		if len(tags) > 0 {
			tagConditions := []string{}
			for _, tag := range tags {
				tagConditions = append(tagConditions, "tags LIKE ?")
				args = append(args, "%"+tag+"%")
			}
			where = append(where, "("+strings.Join(tagConditions, " OR ")+")")
		}

		if filterPinned {
			where = append(where, "is_pinned = 1")
		}
		if filterStarred {
			where = append(where, "is_starred = 1")
		}

		whereClause := "WHERE " + strings.Join(where, " AND ")

		orderBy := "created_at DESC"
		switch sortBy {
		case "created_at_asc":
			orderBy = "created_at ASC"
		case "title_asc":
			orderBy = "title ASC"
		case "title_desc":
			orderBy = "title DESC"
		}

		// Get total notes count
		var totalNotes int
		err := db.QueryRow("SELECT COUNT(*) FROM notes WHERE user_id = ?", userID).Scan(&totalNotes)
		if err != nil {
			http.Error(w, "Failed to count notes", http.StatusInternalServerError)
			log.Println("Count error:", err)
			return
		}

		// Get pinned count
		var pinnedCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notes WHERE user_id = ? AND is_pinned = 1", userID).Scan(&pinnedCount)
		if err != nil {
			http.Error(w, "Failed to count pinned notes", http.StatusInternalServerError)
			log.Println("Count error:", err)
			return
		}

		// Get starred count
		var starredCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notes WHERE user_id = ? AND is_starred = 1", userID).Scan(&starredCount)
		if err != nil {
			http.Error(w, "Failed to count starred notes", http.StatusInternalServerError)
			log.Println("Count error:", err)
			return
		}

		// Get filtered count
		var totalCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notes "+whereClause, args...).Scan(&totalCount)
		if err != nil {
			http.Error(w, "Failed to count notes", http.StatusInternalServerError)
			log.Println("Count error:", err)
			return
		}

		argsWithLimit := append(args, pageSize, offset)
		rows, err := db.Query(`
			SELECT id, user_id, title, content, tags, is_pinned, is_starred, created_at, updated_at
			FROM notes `+whereClause+`
			ORDER BY `+orderBy+` LIMIT ? OFFSET ?`, argsWithLimit...)
		if err != nil {
			http.Error(w, "Failed to fetch notes", http.StatusInternalServerError)
			log.Println("Query error:", err)
			return
		}
		defer rows.Close()

		var notes []models.Note
		for rows.Next() {
			var n models.Note

			if err := rows.Scan(&n.ID, &n.UserID, &n.Title, &n.Content, &n.Tags, &n.IsPinned, &n.IsStarred, &n.CreatedAt, &n.UpdatedAt); err == nil {
				n.Content, err = encryptionSvc.Decrypt(n.Content)
				if err != nil {
					http.Error(w, "Failed to decrypt note", http.StatusInternalServerError)
					return
				}

				notes = append(notes, n)
			}
		}

		tagMap := map[string]int{}
		tagRows, _ := db.Query("SELECT tags FROM notes WHERE user_id = ?", userID)
		defer tagRows.Close()
		for tagRows.Next() {
			var tagStr string
			if err := tagRows.Scan(&tagStr); err == nil {
				for _, t := range strings.Split(tagStr, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tagMap[t]++
					}
				}
			}
		}

		// Template functions
		funcMap := template.FuncMap{
			"split":    strings.Split,
			"add":      func(a, b int) int { return a + b },
			"sub":      func(a, b int) int { return a - b },
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
			"len":      func(slice []models.Note) int { return len(slice) },
			"removeQueryParam": func(param string) string {
				q := r.URL.Query()
				q.Del(param)
				return "?" + q.Encode()
			},
			"addQueryParam": func(param, value string) string {
				q := r.URL.Query()
				q.Set(param, value)
				q.Del("page") // Reset page to 1 when applying a filter
				return "?" + q.Encode()
			},
			"toggleTag": func(tag string) string {
				q := r.URL.Query()
				tags := q["tag"]
				found := false
				for i, t := range tags {
					if t == tag {
						tags = append(tags[:i], tags[i+1:]...)
						found = true
						break
					}
				}
				if !found {
					tags = append(tags, tag)
				}
				q.Del("tag")
				for _, t := range tags {
					q.Add("tag", t)
				}
				q.Del("page") // Reset page to 1 when changing tags
				return "?" + q.Encode()
			},
			"containsTag": func(tag string) bool {
				for _, t := range tags {
					if t == tag {
						return true
					}
				}
				return false
			},
			"addPageParam": func(newPage int) string {
				q := r.URL.Query()
				q.Set("page", strconv.Itoa(newPage))
				return "?" + q.Encode()
			},
		}

		tmpl := template.Must(template.New("dashboard.html").Funcs(funcMap).ParseFiles("templates/dashboard.html", "templates/base.html"))

		err = tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
			"Notes":           notes,
			"TotalNotes":      totalNotes,
			"PinnedCount":     pinnedCount,
			"StarredCount":    starredCount,
			"Search":          search,
			"SelectedTags":    tags,
			"TagCloud":        tagMap,
			"FilterPinned":    filterPinned,
			"FilterStarred":   filterStarred,
			"SortBy":          sortBy,
			"Page":            page,
			"TotalPages":      (totalCount + pageSize - 1) / pageSize,
			"IsAuthenticated": isAuthenticated,
		})
		if err != nil {
			log.Println("Template render error:", err)
		}
	}
}

func NewNoteHandler(db *sql.DB, stripeSvc *stripe.Service, encryptionSvc *encryption.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var isAuthenticated bool

		userID := auth.GetUserIDFromContext(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {
			isAuthenticated = true
		}

		noteLimitExceeded, remainingSeconds, isSubscribed, _ := stripeSvc.CheckUserLimits(db, userID)
		if noteLimitExceeded {
			http.Redirect(w, r, "/subscription?limit=notes", http.StatusSeeOther)
			return
		}

		tmpl := template.Must(template.New("note_form.html").Funcs(template.FuncMap{
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		}).ParseFiles("templates/note_form.html", "templates/base.html"))

		if r.Method == http.MethodGet {
			err := tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
				"IsSubscribed":     isSubscribed,
				"RemainingSeconds": remainingSeconds,
				"IsAuthenticated":  isAuthenticated,
			})

			if err != nil {
				log.Printf("Template error: %v", err)
				http.Error(w, "Error rendering template", http.StatusInternalServerError)
			}
			return
		}

		title := r.FormValue("title")
		content := r.FormValue("content")
		tags := r.FormValue("tags")
		isPinned := r.FormValue("is_pinned") == "on"
		isStarred := r.FormValue("is_starred") == "on"

		encryptedContent, err := encryptionSvc.Encrypt(content)
		if err != nil {
			http.Error(w, "Failed to encrypt note", http.StatusInternalServerError)
			return
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Insert note
		_, err = tx.Exec(`INSERT INTO notes (user_id, title, content, tags, is_pinned, is_starred, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`,
			userID, title, encryptedContent, tags, isPinned, isStarred)
		if err != nil {
			http.Error(w, "Failed to save note", http.StatusInternalServerError)
			return
		}

		// Update note count
		_, err = tx.Exec(`INSERT INTO user_limits (user_id, note_count)
						VALUES (?, 1)
						ON DUPLICATE KEY UPDATE note_count = note_count + 1`,
			userID)
		if err != nil {
			http.Error(w, "Failed to update note count", http.StatusInternalServerError)
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func EditNoteHandler(db *sql.DB, stripeSvc *stripe.Service, encryptionSvc *encryption.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var isAuthenticated bool

		userID := auth.GetUserIDFromContext(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {
			isAuthenticated = true
		}

		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"split":    strings.Split,
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
			"safeJS":   func(s string) template.JS { return template.JS(s) },
		}).ParseFiles(
			"templates/base.html",
			"templates/note_form.html",
		)
		if err != nil {
			http.Error(w, "Failed to load templates", http.StatusInternalServerError)
			return
		}

		_, remainingSeconds, isSubscribed, _ := stripeSvc.CheckUserLimits(db, userID)

		vars := mux.Vars(r)
		noteID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if r.Method == http.MethodGet {
			var note models.Note

			err := db.QueryRow(`
                SELECT title, content, tags, is_pinned, is_starred 
                FROM notes 
                WHERE id = ? AND user_id = ?`,
				noteID, userID,
			).Scan(&note.Title, &note.Content, &note.Tags, &note.IsPinned, &note.IsStarred)

			if err != nil {
				if err == sql.ErrNoRows {
					http.NotFound(w, r)
				} else {
					http.Error(w, "Failed to fetch note", http.StatusInternalServerError)
				}
				return
			}

			note.Content, err = encryptionSvc.Decrypt(note.Content)
			if err != nil {
				http.Error(w, "Failed to decrypt note", http.StatusInternalServerError)
				return
			}

			err = tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
				"Title":            note.Title,
				"Content":          template.HTML(note.Content),
				"Tags":             note.Tags,
				"IsPinned":         note.IsPinned,
				"IsStarred":        note.IsStarred,
				"RemainingSeconds": remainingSeconds,
				"IsSubscribed":     isSubscribed,
				"IsAuthenticated":  isAuthenticated,
			})
			if err != nil {
				http.Error(w, "Failed to render template", http.StatusInternalServerError)
			}
			return
		}

		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Invalid form data", http.StatusBadRequest)
				return
			}

			title := r.FormValue("title")
			content := r.FormValue("content")
			tags := r.FormValue("tags")
			isPinned := r.FormValue("is_pinned") == "on"
			isStarred := r.FormValue("is_starred") == "on"

			encryptedContent, err := encryptionSvc.Encrypt(content)
			if err != nil {
				http.Error(w, "Failed to encrypt note", http.StatusInternalServerError)
				return
			}

			if title == "" || content == "" {
				http.Error(w, "Title and content are required", http.StatusBadRequest)
				return
			}

			_, err = db.Exec(`
                UPDATE notes 
                SET 
                    title = ?,
                    content = ?,
                    tags = ?,
                    is_pinned = ?,
                    is_starred = ?,
                    updated_at = CURRENT_TIMESTAMP
                WHERE id = ? AND user_id = ?`,
				title, encryptedContent, tags, isPinned, isStarred, noteID, userID,
			)

			if err != nil {
				http.Error(w, "Failed to update note", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func DeleteNoteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserIDFromContext(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		noteID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		_, err = db.Exec("DELETE FROM notes WHERE id=? AND user_id=?", noteID, userID)
		if err != nil {
			http.Error(w, "Failed to delete note", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func ViewNoteHandler(db *sql.DB, encryptionSvc *encryption.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var isAuthenticated bool

		userID := auth.GetUserIDFromContext(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {
			isAuthenticated = true
		}
		vars := mux.Vars(r)
		noteID, err := strconv.Atoi(vars["id"])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		var note models.Note
		err = db.QueryRow(`
            SELECT id, user_id, title, content, tags, is_pinned, is_starred, created_at, updated_at
            FROM notes 
            WHERE id = ? AND user_id = ?`,
			noteID, userID,
		).Scan(&note.ID, &note.UserID, &note.Title, &note.Content, &note.Tags, &note.IsPinned, &note.IsStarred, &note.CreatedAt, &note.UpdatedAt)

		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
			} else {
				http.Error(w, "Failed to fetch note", http.StatusInternalServerError)
				log.Println("View note error:", err)
			}
			return
		}

		note.Content, err = encryptionSvc.Decrypt(note.Content)
		if err != nil {
			http.Error(w, "Failed to decrypt note", http.StatusInternalServerError)
			return
		}

		tmpl := template.Must(template.New("view.html").Funcs(template.FuncMap{
			"split":    strings.Split,
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		}).ParseFiles("templates/view.html", "templates/base.html"))

		err = tmpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
			"Note":            note,
			"IsAuthenticated": isAuthenticated,
		})
		if err != nil {
			log.Println("Template render error:", err)
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
	}
}
