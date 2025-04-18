package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Note struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type DB struct {
	conn *sql.DB
}

func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(`CREATE TABLE IF NOT EXISTS notes (
        id SERIAL PRIMARY KEY,
        title TEXT,
        content TEXT
    )`)
	if err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}

func (d *DB) GetNote(id int) (Note, error) {
	var note Note
	err := d.conn.QueryRow("SELECT id, title, content FROM notes WHERE id = $1", id).Scan(&note.ID, &note.Title, &note.Content)
	if err == sql.ErrNoRows {
		return Note{}, fmt.Errorf("note not found")
	} else if err != nil {
		return Note{}, err
	}
	return note, nil
}

func (d *DB) CreateNote(title, content string) (int, error) {
	var id int
	err := d.conn.QueryRow("INSERT INTO notes (title, content) VALUES ($1, $2) RETURNING id", title, content).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (d *DB) UpdateNote(id int, title, content string) error {
	res, err := d.conn.Exec("UPDATE notes SET title = $1, content = $2 WHERE id = $3", title, content, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("note not found")
	}
	return nil
}

func (d *DB) DeleteNote(id int) error {
	res, err := d.conn.Exec("DELETE FROM notes WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("note not found")
	}
	return nil
}

func (d *DB) SearchNotes(query string) ([]Note, error) {
	var rows *sql.Rows
	var err error

	if query == "" {
		rows, err = d.conn.Query("SELECT id, title, content FROM notes")
	} else {
		rows, err = d.conn.Query(
			"SELECT id, title, content FROM notes WHERE title ILIKE $1 OR content ILIKE $1",
			"%"+query+"%",
		)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.Title, &note.Content); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}
