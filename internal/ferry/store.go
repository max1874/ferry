package ferry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS messages (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE CHECK(length(id) = 32),
    kind TEXT NOT NULL CHECK(kind IN ('text', 'file')),
    sender_name TEXT NOT NULL,
    text_body TEXT,
    file_name TEXT,
    media_type TEXT,
    file_size INTEGER,
    blob_name TEXT UNIQUE,
    created_at TEXT NOT NULL,
    CHECK (
        (kind = 'text' AND text_body IS NOT NULL AND file_name IS NULL AND media_type IS NULL AND file_size IS NULL AND blob_name IS NULL)
        OR
        (kind = 'file' AND text_body IS NULL AND file_name IS NOT NULL AND media_type IS NOT NULL AND file_size IS NOT NULL AND file_size >= 0 AND blob_name IS NOT NULL)
    )
);
CREATE INDEX IF NOT EXISTS messages_created_at ON messages(created_at);
`

type Store struct {
	db       *sql.DB
	blobsDir string
}

func OpenStore(ctx context.Context, dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	blobsDir := filepath.Join(dataDir, "blobs")
	if err := os.MkdirAll(blobsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create blob directory: %w", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dataDir, "ferry.db"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db, blobsDir: blobsDir}
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) initialize(ctx context.Context) error {
	statements := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		schema,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize database: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateText(ctx context.Context, senderName, text string) (Message, error) {
	senderName, err := normalizeSenderName(senderName)
	if err != nil {
		return Message{}, err
	}
	if err := validateText(text); err != nil {
		return Message{}, err
	}
	id, err := newID()
	if err != nil {
		return Message{}, err
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	var sequence int64
	err = s.db.QueryRowContext(ctx, `
        INSERT INTO messages (id, kind, sender_name, text_body, created_at)
        VALUES (?, 'text', ?, ?, ?)
		RETURNING sequence
	`, id, senderName, text, createdAt).Scan(&sequence)
	if err != nil {
		return Message{}, fmt.Errorf("insert text message: %w", err)
	}
	return Message{
		ID:         id,
		Sequence:   sequence,
		Kind:       KindText,
		SenderName: senderName,
		CreatedAt:  createdAt,
		Text:       &text,
	}, nil
}

func (s *Store) CreateFile(ctx context.Context, senderName, fileName, mediaType string, source io.Reader) (Message, error) {
	senderName, err := normalizeSenderName(senderName)
	if err != nil {
		return Message{}, err
	}
	fileName, err = normalizeFileName(fileName)
	if err != nil {
		return Message{}, err
	}
	mediaType = normalizeMediaType(mediaType)

	id, err := newID()
	if err != nil {
		return Message{}, err
	}
	blobName := id + ".blob"
	finalPath := filepath.Join(s.blobsDir, blobName)
	file, err := os.OpenFile(finalPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Message{}, fmt.Errorf("create blob: %w", err)
	}
	keepFile := false
	defer func() {
		if !keepFile {
			os.Remove(finalPath)
		}
	}()

	written, copyErr := io.Copy(file, io.LimitReader(source, MaxFileBytes+1))
	if copyErr != nil {
		file.Close()
		return Message{}, fmt.Errorf("write blob: %w", copyErr)
	}
	if written > MaxFileBytes {
		file.Close()
		return Message{}, fmt.Errorf("%w: file exceeds %d bytes", ErrTooLarge, MaxFileBytes)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return Message{}, fmt.Errorf("sync blob: %w", err)
	}
	if err := file.Close(); err != nil {
		return Message{}, fmt.Errorf("close blob: %w", err)
	}

	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	var sequence int64
	err = s.db.QueryRowContext(ctx, `
        INSERT INTO messages (id, kind, sender_name, file_name, media_type, file_size, blob_name, created_at)
        VALUES (?, 'file', ?, ?, ?, ?, ?, ?)
		RETURNING sequence
	`, id, senderName, fileName, mediaType, written, blobName, createdAt).Scan(&sequence)
	if err != nil {
		return Message{}, fmt.Errorf("insert file message: %w", err)
	}
	keepFile = true
	return Message{
		ID:         id,
		Sequence:   sequence,
		Kind:       KindFile,
		SenderName: senderName,
		CreatedAt:  createdAt,
		File: &FileInfo{
			Name:        fileName,
			MediaType:   mediaType,
			Size:        written,
			DownloadURL: "/api/v1/files/" + id,
		},
	}, nil
}

func (s *Store) ListMessages(ctx context.Context, after int64, limit int) ([]Message, int64, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT sequence, id, kind, sender_name, text_body, file_name, media_type, file_size, blob_name, created_at
        FROM messages
        WHERE sequence > ?
        ORDER BY sequence ASC
        LIMIT ?
    `, after, limit)
	if err != nil {
		return nil, after, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0)
	nextCursor := after
	for rows.Next() {
		message, _, err := scanMessage(rows)
		if err != nil {
			return nil, after, err
		}
		messages = append(messages, message)
		nextCursor = message.Sequence
	}
	if err := rows.Err(); err != nil {
		return nil, after, fmt.Errorf("iterate messages: %w", err)
	}
	return messages, nextCursor, nil
}

func (s *Store) OpenFile(ctx context.Context, id string) (Message, *os.File, error) {
	if !validID(id) {
		return Message{}, nil, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `
        SELECT sequence, id, kind, sender_name, text_body, file_name, media_type, file_size, blob_name, created_at
        FROM messages
        WHERE id = ?
    `, id)
	message, blobName, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, nil, ErrNotFound
	}
	if err != nil {
		return Message{}, nil, err
	}
	if message.Kind != KindFile {
		return Message{}, nil, ErrNotFound
	}
	if blobName != id+".blob" {
		return Message{}, nil, fmt.Errorf("stored file metadata is inconsistent")
	}
	file, err := os.Open(filepath.Join(s.blobsDir, blobName))
	if errors.Is(err, os.ErrNotExist) {
		return Message{}, nil, ErrNotFound
	}
	if err != nil {
		return Message{}, nil, fmt.Errorf("open blob: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return Message{}, nil, fmt.Errorf("stat blob: %w", err)
	}
	if info.Size() != message.File.Size {
		file.Close()
		return Message{}, nil, fmt.Errorf("stored blob size does not match metadata")
	}
	return message, file, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMessage(row rowScanner) (Message, string, error) {
	var (
		message   Message
		kind      string
		text      sql.NullString
		fileName  sql.NullString
		mediaType sql.NullString
		fileSize  sql.NullInt64
		blobName  sql.NullString
	)
	if err := row.Scan(
		&message.Sequence,
		&message.ID,
		&kind,
		&message.SenderName,
		&text,
		&fileName,
		&mediaType,
		&fileSize,
		&blobName,
		&message.CreatedAt,
	); err != nil {
		return Message{}, "", err
	}

	switch Kind(kind) {
	case KindText:
		if !text.Valid || fileName.Valid || mediaType.Valid || fileSize.Valid || blobName.Valid {
			return Message{}, "", fmt.Errorf("stored text metadata is inconsistent")
		}
		message.Kind = KindText
		message.Text = &text.String
	case KindFile:
		if text.Valid || !fileName.Valid || !mediaType.Valid || !fileSize.Valid || !blobName.Valid {
			return Message{}, "", fmt.Errorf("stored file metadata is inconsistent")
		}
		message.Kind = KindFile
		message.File = &FileInfo{
			Name:        fileName.String,
			MediaType:   mediaType.String,
			Size:        fileSize.Int64,
			DownloadURL: "/api/v1/files/" + message.ID,
		}
	default:
		return Message{}, "", fmt.Errorf("unknown stored message kind %q", kind)
	}
	if err := validateStoredMessage(message, blobName.String); err != nil {
		return Message{}, "", err
	}
	return message, blobName.String, nil
}

func validateStoredMessage(message Message, blobName string) error {
	if !validID(message.ID) {
		return fmt.Errorf("stored message id is invalid")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, message.CreatedAt)
	if err != nil || createdAt.UTC().Format(time.RFC3339Nano) != message.CreatedAt {
		return fmt.Errorf("stored message timestamp is not canonical UTC RFC 3339")
	}
	senderName, err := normalizeSenderName(message.SenderName)
	if err != nil || senderName != message.SenderName {
		return fmt.Errorf("stored sender name is invalid")
	}
	switch message.Kind {
	case KindText:
		if message.Text == nil || validateText(*message.Text) != nil || message.File != nil || blobName != "" {
			return fmt.Errorf("stored text message is invalid")
		}
	case KindFile:
		if message.Text != nil || message.File == nil || message.File.Size < 0 || message.File.Size > MaxFileBytes {
			return fmt.Errorf("stored file message is invalid")
		}
		fileName, err := normalizeFileName(message.File.Name)
		if err != nil || fileName != message.File.Name || normalizeMediaType(message.File.MediaType) != message.File.MediaType || blobName != message.ID+".blob" {
			return fmt.Errorf("stored file message is invalid")
		}
	default:
		return fmt.Errorf("unknown stored message kind %q", message.Kind)
	}
	return nil
}
