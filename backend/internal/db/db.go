// Package db wraps a small SQLite store for file metadata.
// It uses the pure-Go modernc.org/sqlite driver so the binary can be built
// without CGO and shipped in a minimal container image.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"memorydrive/internal/models"

	_ "modernc.org/sqlite"
)

// Store is a thin data-access layer over SQLite.
type Store struct {
	db *sql.DB
}

// Open opens (and if needed creates) the SQLite database at path and ensures
// the schema exists.
func Open(path string) (*Store, error) {
	// _pragma options improve concurrency and durability for our workload.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer at a time; keep the pool small and predictable.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: sqlDB}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS files (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    kind         TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size         INTEGER NOT NULL,
    storage_path TEXT NOT NULL DEFAULT '',
    text_content TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

// Insert stores a new file record.
func (s *Store) Insert(f *models.File) error {
	_, err := s.db.Exec(
		`INSERT INTO files (id, name, kind, content_type, size, storage_path, text_content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Name, string(f.Kind), f.ContentType, f.Size, f.StoragePath, f.TextContent, f.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert file: %w", err)
	}
	return nil
}

// Get returns a single file by id, or (nil, nil) if it does not exist.
func (s *Store) Get(id string) (*models.File, error) {
	row := s.db.QueryRow(
		`SELECT id, name, kind, content_type, size, storage_path, text_content, created_at
		 FROM files WHERE id = ?`, id)
	return scanFile(row)
}

// List returns all files, optionally filtered by a case-insensitive search
// term that matches against name or text content. Newest first.
func (s *Store) List(search string) ([]*models.File, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if search == "" {
		rows, err = s.db.Query(
			`SELECT id, name, kind, content_type, size, storage_path, text_content, created_at
			 FROM files ORDER BY created_at DESC`)
	} else {
		like := "%" + search + "%"
		rows, err = s.db.Query(
			`SELECT id, name, kind, content_type, size, storage_path, text_content, created_at
			 FROM files
			 WHERE name LIKE ? COLLATE NOCASE OR text_content LIKE ? COLLATE NOCASE
			 ORDER BY created_at DESC`, like, like)
	}
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var out []*models.File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Delete removes a file record and returns whether a row was deleted.
func (s *Store) Delete(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM files WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete file: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Stats returns aggregate counts used by the /stats endpoint.
func (s *Store) Stats() (count int64, totalSize int64, err error) {
	row := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size), 0) FROM files`)
	if err = row.Scan(&count, &totalSize); err != nil {
		return 0, 0, fmt.Errorf("stats: %w", err)
	}
	return count, totalSize, nil
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanFile(sc scanner) (*models.File, error) {
	var (
		f       models.File
		kind    string
		created time.Time
	)
	err := sc.Scan(&f.ID, &f.Name, &kind, &f.ContentType, &f.Size, &f.StoragePath, &f.TextContent, &created)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}
	f.Kind = models.Kind(kind)
	f.CreatedAt = created
	return &f, nil
}
