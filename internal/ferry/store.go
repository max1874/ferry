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
CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY CHECK(length(id) = 32),
    name TEXT NOT NULL,
    token_hash BLOB NOT NULL UNIQUE CHECK(length(token_hash) = 32),
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS access_settings (
    singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
    salt BLOB NOT NULL CHECK(length(salt) = 16),
    verifier BLOB NOT NULL CHECK(length(verifier) = 32),
    iterations INTEGER NOT NULL CHECK(iterations > 0)
);
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

func (s *Store) AccessPassword(ctx context.Context) (*AccessPasswordVerifier, error) {
	var salt, hash []byte
	var iterations int
	err := s.db.QueryRowContext(ctx, `
        SELECT salt, verifier, iterations FROM access_settings WHERE singleton = 1
    `).Scan(&salt, &hash, &iterations)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read access password: %w", err)
	}
	if len(salt) != accessPasswordSaltBytes || len(hash) != accessPasswordHashBytes || iterations <= 0 {
		return nil, fmt.Errorf("stored access password is invalid")
	}
	return &AccessPasswordVerifier{Salt: salt, Hash: hash, Iterations: iterations}, nil
}

func (s *Store) SetAccessPassword(ctx context.Context, verifier *AccessPasswordVerifier) error {
	return s.setAccessPassword(ctx, "", verifier)
}

func (s *Store) SetAccessPasswordForDevice(ctx context.Context, requesterID string, verifier *AccessPasswordVerifier) error {
	if !validID(requesterID) {
		return ErrUnauthorized
	}
	return s.setAccessPassword(ctx, requesterID, verifier)
}

func (s *Store) setAccessPassword(ctx context.Context, requesterID string, verifier *AccessPasswordVerifier) error {
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin access password update: %w", err)
	}
	defer transaction.Rollback()
	if requesterID != "" {
		var exists int
		if err := transaction.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)", requesterID).Scan(&exists); err != nil {
			return fmt.Errorf("check access password requester: %w", err)
		}
		if exists == 0 {
			return ErrUnauthorized
		}
	}
	if verifier == nil {
		if _, err := transaction.ExecContext(ctx, "DELETE FROM access_settings WHERE singleton = 1"); err != nil {
			return fmt.Errorf("disable access password: %w", err)
		}
		return transaction.Commit()
	}
	if len(verifier.Salt) != accessPasswordSaltBytes || len(verifier.Hash) != accessPasswordHashBytes || verifier.Iterations <= 0 {
		return fmt.Errorf("access password verifier is invalid")
	}
	_, err = transaction.ExecContext(ctx, `
        INSERT INTO access_settings (singleton, salt, verifier, iterations)
        VALUES (1, ?, ?, ?)
        ON CONFLICT(singleton) DO UPDATE SET
            salt = excluded.salt,
            verifier = excluded.verifier,
            iterations = excluded.iterations
    `, verifier.Salt, verifier.Hash, verifier.Iterations)
	if err != nil {
		return fmt.Errorf("save access password: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit access password: %w", err)
	}
	return nil
}

func (s *Store) DeviceCount(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices").Scan(&count); err != nil {
		return 0, fmt.Errorf("count devices: %w", err)
	}
	return count, nil
}

func (s *Store) CreateDevice(ctx context.Context, name string, tokenHash [32]byte) (Device, error) {
	return s.createDevice(ctx, name, tokenHash, "")
}

func (s *Store) CreateDeviceForIssuer(ctx context.Context, name string, tokenHash [32]byte, issuerID string) (Device, error) {
	if !validID(issuerID) {
		return Device{}, ErrNotFound
	}
	return s.createDevice(ctx, name, tokenHash, issuerID)
}

func (s *Store) createDevice(ctx context.Context, name string, tokenHash [32]byte, issuerID string) (Device, error) {
	name, err := normalizeSenderName(name)
	if err != nil {
		return Device{}, err
	}
	id, err := newID()
	if err != nil {
		return Device{}, err
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	query := `INSERT INTO devices (id, name, token_hash, created_at) VALUES (?, ?, ?, ?)`
	arguments := []any{id, name, tokenHash[:], createdAt}
	if issuerID != "" {
		query = `
            INSERT INTO devices (id, name, token_hash, created_at)
            SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM devices WHERE id = ?)
        `
		arguments = append(arguments, issuerID)
	}
	result, err := s.db.ExecContext(ctx, query, arguments...)
	if err != nil {
		return Device{}, fmt.Errorf("insert device: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return Device{}, fmt.Errorf("read device insert result: %w", err)
	}
	if inserted != 1 {
		return Device{}, ErrNotFound
	}
	return Device{ID: id, Name: name, CreatedAt: createdAt}, nil
}

func (s *Store) DeviceExists(ctx context.Context, id string) (bool, error) {
	if !validID(id) {
		return false, nil
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)", id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check device: %w", err)
	}
	return exists == 1, nil
}

func (s *Store) AuthenticateDevice(ctx context.Context, tokenHash [32]byte) (Device, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT id, name, created_at FROM devices WHERE token_hash = ?
    `, tokenHash[:])
	device, err := scanDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return device, err
}

func (s *Store) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, name, created_at FROM devices ORDER BY created_at, id
    `)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()
	devices := make([]Device, 0)
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices: %w", err)
	}
	return devices, nil
}

func (s *Store) DeleteDevice(ctx context.Context, requesterID, id string) error {
	if !validID(requesterID) || !validID(id) {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx, `
        DELETE FROM devices
        WHERE id = ? AND id <> ? AND (SELECT COUNT(*) FROM devices) > 1
          AND EXISTS (SELECT 1 FROM devices WHERE id = ?)
	`, id, requesterID, requesterID)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result: %w", err)
	}
	if deleted != 1 {
		var targetExists, requesterExists int
		if err := s.db.QueryRowContext(ctx, `
            SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?),
                   EXISTS(SELECT 1 FROM devices WHERE id = ?)
        `, id, requesterID).Scan(&targetExists, &requesterExists); err != nil {
			return fmt.Errorf("check device after delete: %w", err)
		}
		if requesterExists == 0 {
			return ErrUnauthorized
		}
		if targetExists == 0 {
			return ErrNotFound
		}
		return ErrInvalid
	}
	return nil
}

func scanDevice(row rowScanner) (Device, error) {
	var device Device
	if err := row.Scan(&device.ID, &device.Name, &device.CreatedAt); err != nil {
		return Device{}, err
	}
	if !validID(device.ID) {
		return Device{}, fmt.Errorf("stored device id is invalid")
	}
	name, err := normalizeSenderName(device.Name)
	if err != nil || name != device.Name {
		return Device{}, fmt.Errorf("stored device name is invalid")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, device.CreatedAt)
	if err != nil || createdAt.UTC().Format(time.RFC3339Nano) != device.CreatedAt {
		return Device{}, fmt.Errorf("stored device timestamp is not canonical UTC RFC 3339")
	}
	return device, nil
}

func (s *Store) CreateText(ctx context.Context, senderName, text string) (Message, error) {
	return s.createText(ctx, "", senderName, text)
}

func (s *Store) CreateTextForDevice(ctx context.Context, device Device, text string) (Message, error) {
	if !validID(device.ID) {
		return Message{}, ErrUnauthorized
	}
	return s.createText(ctx, device.ID, device.Name, text)
}

func (s *Store) createText(ctx context.Context, requesterID, senderName, text string) (Message, error) {
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
	query := `
        INSERT INTO messages (id, kind, sender_name, text_body, created_at)
        VALUES (?, 'text', ?, ?, ?)
        RETURNING sequence
    `
	arguments := []any{id, senderName, text, createdAt}
	if requesterID != "" {
		query = `
            INSERT INTO messages (id, kind, sender_name, text_body, created_at)
            SELECT ?, 'text', ?, ?, ?
            WHERE EXISTS (SELECT 1 FROM devices WHERE id = ? AND name = ?)
            RETURNING sequence
        `
		arguments = append(arguments, requesterID, senderName)
	}
	err = s.db.QueryRowContext(ctx, query, arguments...).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) && requesterID != "" {
		return Message{}, ErrUnauthorized
	}
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
	return s.createFile(ctx, "", senderName, fileName, mediaType, source)
}

func (s *Store) CreateFileForDevice(ctx context.Context, device Device, fileName, mediaType string, source io.Reader) (Message, error) {
	if !validID(device.ID) {
		return Message{}, ErrUnauthorized
	}
	return s.createFile(ctx, device.ID, device.Name, fileName, mediaType, source)
}

func (s *Store) createFile(ctx context.Context, requesterID, senderName, fileName, mediaType string, source io.Reader) (Message, error) {
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
	query := `
        INSERT INTO messages (id, kind, sender_name, file_name, media_type, file_size, blob_name, created_at)
        VALUES (?, 'file', ?, ?, ?, ?, ?, ?)
        RETURNING sequence
    `
	arguments := []any{id, senderName, fileName, mediaType, written, blobName, createdAt}
	if requesterID != "" {
		query = `
            INSERT INTO messages (id, kind, sender_name, file_name, media_type, file_size, blob_name, created_at)
            SELECT ?, 'file', ?, ?, ?, ?, ?, ?
            WHERE EXISTS (SELECT 1 FROM devices WHERE id = ? AND name = ?)
            RETURNING sequence
        `
		arguments = append(arguments, requesterID, senderName)
	}
	err = s.db.QueryRowContext(ctx, query, arguments...).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) && requesterID != "" {
		return Message{}, ErrUnauthorized
	}
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
