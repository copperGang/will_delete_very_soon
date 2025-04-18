package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"notes-app/pkg/db"
)

type Note struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func makeGetNoteHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil || id <= 0 {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		note, err := db.GetNote(id)
		if err != nil {
			if err.Error() == "note not found" {
				http.Error(w, "Note not found", http.StatusNotFound)
			} else {
				log.Printf("Error fetching note: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(note)
	}
}

func makeCreateNoteHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var note Note
		if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if note.Title == "" || note.Content == "" {
			http.Error(w, "Title and content required", http.StatusBadRequest)
			return
		}
		id, err := db.CreateNote(note.Title, note.Content)
		if err != nil {
			log.Printf("Error creating note: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Note{
			ID:      id,
			Title:   note.Title,
			Content: note.Content,
		})
	}
}

func makeUpdateNoteHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil || id <= 0 {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		var note Note
		if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if note.Title == "" || note.Content == "" {
			http.Error(w, "Title and content required", http.StatusBadRequest)
			return
		}
		err = db.UpdateNote(id, note.Title, note.Content)
		if err != nil {
			if err.Error() == "note not found" {
				http.Error(w, "Note not found", http.StatusNotFound)
			} else {
				log.Printf("Error updating note: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func makeDeleteNoteHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil || id <= 0 {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		err = db.DeleteNote(id)
		if err != nil {
			if err.Error() == "note not found" {
				http.Error(w, "Note not found", http.StatusNotFound)
			} else {
				log.Printf("Error deleting note: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func makeSearchNotesHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "Missing query parameter 'q'", http.StatusBadRequest)
			return
		}
		notes, err := db.SearchNotes(q)
		if err != nil {
			log.Printf("Error searching notes: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		var convertedNotes []Note
		for _, dbNote := range notes {
			convertedNotes = append(convertedNotes, Note{
				ID:      dbNote.ID,
				Title:   dbNote.Title,
				Content: dbNote.Content,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]Note{
			"search_result": convertedNotes,
		})
	}
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <address> <dsn>")
		return
	}
	address := os.Args[1]
	dsn := os.Args[2]

	dbConn, err := db.NewDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/notes/{id}", makeGetNoteHandler(dbConn)).Methods("GET")
	router.HandleFunc("/api/v1/notes", makeCreateNoteHandler(dbConn)).Methods("POST")
	router.HandleFunc("/api/v1/notes/{id}", makeUpdateNoteHandler(dbConn)).Methods("PUT")
	router.HandleFunc("/api/v1/notes/{id}", makeDeleteNoteHandler(dbConn)).Methods("DELETE")
	router.HandleFunc("/api/v1/notes/search", makeSearchNotesHandler(dbConn)).Methods("GET")

	srv := &http.Server{
		Addr:    address,
		Handler: router,
	}

	go func() {
		log.Printf("Server starting on %s", address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited.")
}
