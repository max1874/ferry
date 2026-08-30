package ferry

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLegacyDatabaseMigratesDeviceKindsAndInfersHistory(t *testing.T) {
	dataDir := t.TempDir()
	database, err := sql.Open("sqlite", filepath.Join(dataDir, "ferry.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`
        CREATE TABLE devices (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            token_hash BLOB NOT NULL UNIQUE,
            created_at TEXT NOT NULL
        );
        CREATE TABLE messages (
            sequence INTEGER PRIMARY KEY AUTOINCREMENT,
            id TEXT NOT NULL UNIQUE,
            kind TEXT NOT NULL,
            sender_name TEXT NOT NULL,
            text_body TEXT,
            file_name TEXT,
            media_type TEXT,
            file_size INTEGER,
            blob_name TEXT,
            created_at TEXT NOT NULL
        );
        INSERT INTO devices (id, name, token_hash, created_at)
        VALUES ('0123456789abcdef0123456789abcdef', 'Mac Web', zeroblob(32), '2026-08-29T00:00:00Z');
        INSERT INTO messages (id, kind, sender_name, text_body, created_at)
        VALUES ('fedcba9876543210fedcba9876543210', 'text', 'iPhone 17 Pro', 'legacy', '2026-08-29T00:00:01Z');
    `)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenStore(t.Context(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	var tokenHash [32]byte
	device, err := store.AuthenticateDevice(t.Context(), tokenHash)
	if err != nil || device.Kind != DeviceKindMac {
		t.Fatalf("legacy device = %#v, error = %v", device, err)
	}
	messages, _, err := store.ListMessages(t.Context(), 0, 10)
	if err != nil || len(messages) != 1 || messages[0].SenderKind != DeviceKindIPhone {
		t.Fatalf("legacy messages = %#v, error = %v", messages, err)
	}
	var nullDeviceKind, nullMessageKind bool
	if err := store.db.QueryRow("SELECT kind IS NULL FROM devices LIMIT 1").Scan(&nullDeviceKind); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT sender_kind IS NULL FROM messages LIMIT 1").Scan(&nullMessageKind); err != nil {
		t.Fatal(err)
	}
	if !nullDeviceKind || !nullMessageKind {
		t.Fatal("migration rewrote legacy rows instead of preserving nullable history")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenStore(t.Context(), dataDir)
	if err != nil {
		t.Fatalf("idempotent reopen: %v", err)
	}
	t.Cleanup(func() { reopened.Close() })
	if _, err := reopened.db.Exec("PRAGMA ignore_check_constraints = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.db.Exec("UPDATE devices SET kind = 'car'"); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.AuthenticateDevice(t.Context(), tokenHash); err == nil || !strings.Contains(err.Error(), "stored device kind is invalid") {
		t.Fatalf("corrupt stored device kind error = %v", err)
	}
}

func TestDeviceKindInferenceUsesFiniteFallback(t *testing.T) {
	for _, test := range []struct {
		name string
		want DeviceKind
	}{
		{"iPhone 17 Pro", DeviceKindIPhone},
		{"iPad Web", DeviceKindIPad},
		{"macmini", DeviceKindMac},
		{"Android Web", DeviceKindAndroid},
		{"Windows Web", DeviceKindWindows},
		{"Kitchen Display", DeviceKindBrowser},
	} {
		if got := inferDeviceKind(test.name); got != test.want {
			t.Errorf("inferDeviceKind(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestTextBoundariesAndPreservation(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	tests := []struct {
		name string
		text string
		want error
	}{
		{name: "empty", text: "", want: ErrInvalid},
		{name: "whitespace", text: " \n\t", want: ErrInvalid},
		{name: "maximum", text: "x" + strings.Repeat(" ", MaxTextBytes-1)},
		{name: "over maximum", text: strings.Repeat("x", MaxTextBytes+1), want: ErrTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, err := store.CreateText(ctx, " Web ", test.text)
			if test.want != nil {
				if !errors.Is(err, test.want) {
					t.Fatalf("CreateText() error = %v, want %v", err, test.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateText() error = %v", err)
			}
			if message.Text == nil || *message.Text != test.text {
				t.Fatalf("text was not preserved")
			}
			if message.SenderName != "Web" {
				t.Fatalf("sender = %q, want Web", message.SenderName)
			}
		})
	}
}

func TestFileNameCannotSelectBlobPathAndPersists(t *testing.T) {
	dataDir := t.TempDir()
	ctx := context.Background()
	store, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}

	contents := []byte("ferry file\n")
	message, err := store.CreateFile(ctx, "Web", `..\..\ferry.db`, "text/plain", bytes.NewReader(contents))
	if err != nil {
		t.Fatal(err)
	}
	if message.File == nil || message.File.Name != "ferry.db" {
		t.Fatalf("display name = %#v, want ferry.db", message.File)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != message.ID+".blob" {
		t.Fatalf("blob entries = %v, want server-generated name", entries)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	messages, cursor, err := reopened.ListMessages(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || cursor != message.Sequence || messages[0].ID != message.ID {
		t.Fatalf("reopened messages = %#v, cursor = %d", messages, cursor)
	}
	_, file, err := reopened.OpenFile(ctx, message.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	got, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatalf("download bytes = %q, want %q", got, contents)
	}
}

func TestFileInsertFailureRemovesInstalledBlob(t *testing.T) {
	dataDir := t.TempDir()
	store, err := OpenStore(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateFile(context.Background(), "Web", "hello.txt", "text/plain", strings.NewReader("hello")); err == nil {
		t.Fatal("CreateFile() succeeded after database close")
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("orphan blobs after insert failure: %v", entries)
	}
}

func TestFileReadFailureRemovesPartialBlob(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateFile(context.Background(), "Web", "hello.txt", "text/plain", failingReader{}); err == nil {
		t.Fatal("CreateFile() accepted a failed source")
	}
	entries, err := os.ReadDir(store.blobsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial blobs after read failure: %v", entries)
	}
}

func TestFileSizeBoundaries(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if message, err := store.CreateFile(ctx, "Web", "empty.bin", "application/octet-stream", strings.NewReader("")); err != nil || message.File == nil || message.File.Size != 0 {
		t.Fatalf("empty file message = %#v, error = %v", message, err)
	}
	message, err := store.CreateFile(ctx, "Web", "maximum.bin", "application/octet-stream", io.LimitReader(zeroReader{}, MaxFileBytes))
	if err != nil {
		t.Fatalf("maximum file error = %v", err)
	}
	if message.File == nil || message.File.Size != MaxFileBytes {
		t.Fatalf("maximum file message = %#v", message)
	}
}

func TestDotDotFileNameIsRejected(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateFile(context.Background(), "Web", "..", "text/plain", strings.NewReader("hello")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateFile() error = %v, want ErrInvalid", err)
	}
}

func TestCorruptStoredFileMetadataFailsClosed(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	message, err := store.CreateFile(ctx, "Web", "hello.txt", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE messages SET file_size = ? WHERE id = ?", MaxFileBytes+1, message.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ListMessages(ctx, 0, 100); err == nil || !strings.Contains(err.Error(), "stored file message is invalid") {
		t.Fatalf("ListMessages() error = %v, want invalid stored file", err)
	}
}

func TestBlobSizeMismatchFailsBeforeDownload(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	message, err := store.CreateFile(ctx, "Web", "hello.txt", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.blobsDir, message.ID+".blob"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, file, err := store.OpenFile(ctx, message.ID); err == nil || file != nil || !strings.Contains(err.Error(), "size does not match") {
		t.Fatalf("OpenFile() file = %v, error = %v", file, err)
	}
}

func TestUnknownStoredKindFailsClosed(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `
        INSERT INTO messages (id, kind, sender_name, created_at)
        VALUES ('0123456789abcdef0123456789abcdef', 'link', 'Web', '2026-08-29T00:00:00Z')
    `); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ListMessages(ctx, 0, 100); err == nil || !strings.Contains(err.Error(), "unknown stored message kind") {
		t.Fatalf("ListMessages() error = %v, want unknown kind failure", err)
	}
}

func TestCursorOrdersMessagesIndependentlyOfTimestamp(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	first, err := store.CreateText(ctx, "Web", "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateText(ctx, "Web", "second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE messages SET created_at = '2026-08-29T00:00:00Z'"); err != nil {
		t.Fatal(err)
	}
	messages, cursor, err := store.ListMessages(ctx, first.Sequence, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].ID != second.ID || cursor != second.Sequence {
		t.Fatalf("messages after cursor = %#v, cursor = %d", messages, cursor)
	}
}

func TestDeviceTokenPersistsAsHashAndRevokesImmediately(t *testing.T) {
	dataDir := t.TempDir()
	ctx := context.Background()
	store, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	token, hash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.CreateDevice(ctx, " Max Mac ", hash)
	if err != nil {
		t.Fatal(err)
	}
	if device.Name != "Max Mac" {
		t.Fatalf("device = %#v", device)
	}
	var rawTokenCount int
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE CAST(token_hash AS TEXT) = ?", token).Scan(&rawTokenCount); err != nil {
		t.Fatal(err)
	}
	if rawTokenCount != 0 {
		t.Fatal("raw device token was stored")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	authenticated, err := reopened.AuthenticateDevice(ctx, hash)
	if err != nil || authenticated.ID != device.ID {
		t.Fatalf("authenticated = %#v, error = %v", authenticated, err)
	}
	devices, err := reopened.ListDevices(ctx)
	if err != nil || len(devices) != 1 || devices[0].ID != device.ID {
		t.Fatalf("devices = %#v, error = %v", devices, err)
	}
	_, secondHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := reopened.CreateDevice(ctx, "iPhone", secondHash)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.DeleteDevice(ctx, second.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.AuthenticateDevice(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("authentication after revoke error = %v", err)
	}
	if err := reopened.DeleteDevice(ctx, second.ID, device.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete error = %v", err)
	}
	if err := reopened.DeleteDevice(ctx, second.ID, second.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("self delete error = %v", err)
	}
}

func TestDeviceNameUsesSenderBoundary(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	for _, test := range []struct {
		label string
		name  string
		want  error
	}{
		{label: "maximum", name: strings.Repeat("x", MaxSenderBytes)},
		{label: "over maximum", name: strings.Repeat("x", MaxSenderBytes+1), want: ErrTooLarge},
		{label: "control character", name: "bad\nname", want: ErrInvalid},
	} {
		t.Run(test.label, func(t *testing.T) {
			_, hash, err := newDeviceToken()
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.CreateDevice(ctx, test.name, hash)
			if !errors.Is(err, test.want) {
				t.Fatalf("CreateDevice() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestDeviceCreationRequiresIssuerAtInsertTime(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, ownerHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.CreateDevice(ctx, "Owner", ownerHash)
	if err != nil {
		t.Fatal(err)
	}
	_, issuerHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := store.CreateDevice(ctx, "Issuer", issuerHash)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteDevice(ctx, owner.ID, issuer.ID); err != nil {
		t.Fatal(err)
	}
	_, claimedHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDeviceForIssuer(ctx, "Rejected", claimedHash, issuer.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("creation after issuer revoke error = %v", err)
	}
	if _, err := store.CreateDeviceForIssuer(ctx, "Accepted", claimedHash, owner.ID); err != nil {
		t.Fatalf("creation from live issuer: %v", err)
	}
}

func TestRevokedRequesterCannotDeleteAnotherDevice(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	create := func(name string) Device {
		t.Helper()
		_, hash, err := newDeviceToken()
		if err != nil {
			t.Fatal(err)
		}
		device, err := store.CreateDevice(ctx, name, hash)
		if err != nil {
			t.Fatal(err)
		}
		return device
	}
	first := create("First")
	second := create("Second")
	third := create("Third")
	if err := store.DeleteDevice(ctx, second.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteDevice(ctx, first.ID, second.ID); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked requester delete error = %v", err)
	}
	devices, err := store.ListDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].ID != second.ID || devices[1].ID != third.ID {
		t.Fatalf("devices after stale delete = %#v", devices)
	}
}

func TestMessageKindMustMatchPersistedDeviceKind(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, explicitHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := store.CreateDeviceWithKind(ctx, "Kitchen Display", DeviceKindMac, explicitHash)
	if err != nil {
		t.Fatal(err)
	}
	forged := explicit
	forged.Kind = DeviceKindIPhone
	if _, err := store.CreateTextForDevice(ctx, forged, "must not land"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("forged text kind error = %v, want ErrUnauthorized", err)
	}
	if _, err := store.CreateFileForDevice(ctx, forged, "forged.txt", "text/plain", strings.NewReader("must not land")); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("forged file kind error = %v, want ErrUnauthorized", err)
	}

	_, legacyHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.CreateDevice(ctx, "Legacy iPhone", legacyHash)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE devices SET kind = NULL WHERE id = ?", legacy.ID); err != nil {
		t.Fatal(err)
	}
	legacy, err = store.AuthenticateDevice(ctx, legacyHash)
	if err != nil || legacy.Kind != DeviceKindIPhone {
		t.Fatalf("legacy device = %#v, error = %v", legacy, err)
	}
	if _, err := store.CreateTextForDevice(ctx, legacy, "legacy lands"); err != nil {
		t.Fatalf("legacy inferred kind rejected: %v", err)
	}
	legacy.Kind = DeviceKindMac
	if _, err := store.CreateTextForDevice(ctx, legacy, "forged legacy kind"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("forged legacy kind error = %v, want ErrUnauthorized", err)
	}

	messages, _, err := store.ListMessages(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].SenderKind != DeviceKindIPhone || messages[0].Text == nil || *messages[0].Text != "legacy lands" {
		t.Fatalf("messages after kind checks = %#v", messages)
	}
	entries, err := os.ReadDir(store.blobsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("forged file kind left blobs: %v", entries)
	}
}

func TestRevokedDeviceCannotCreateMessages(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	create := func(name string) Device {
		t.Helper()
		_, hash, err := newDeviceToken()
		if err != nil {
			t.Fatal(err)
		}
		device, err := store.CreateDevice(ctx, name, hash)
		if err != nil {
			t.Fatal(err)
		}
		return device
	}
	revoked := create("Revoked")
	owner := create("Owner")
	if err := store.DeleteDevice(ctx, owner.ID, revoked.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateTextForDevice(ctx, revoked, "must not land"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("CreateTextForDevice() error = %v, want ErrUnauthorized", err)
	}
	if _, err := store.CreateFileForDevice(ctx, revoked, "secret.txt", "text/plain", strings.NewReader("must not land")); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("CreateFileForDevice() error = %v, want ErrUnauthorized", err)
	}
	messages, _, err := store.ListMessages(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("revoked device created messages: %#v", messages)
	}
	entries, err := os.ReadDir(store.blobsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("revoked file upload left blobs: %v", entries)
	}
}

func TestRevokingDeviceDuringFileUploadPreventsMessageAndBlob(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	create := func(name string) Device {
		t.Helper()
		_, hash, err := newDeviceToken()
		if err != nil {
			t.Fatal(err)
		}
		device, err := store.CreateDevice(ctx, name, hash)
		if err != nil {
			t.Fatal(err)
		}
		return device
	}
	uploader := create("Uploader")
	owner := create("Owner")
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := store.CreateFileForDevice(ctx, uploader, "slow.txt", "text/plain", &gatedReader{
			started: started,
			release: release,
			data:    []byte("must not land"),
		})
		result <- err
	}()
	<-started
	if err := store.DeleteDevice(ctx, owner.ID, uploader.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("in-flight upload error = %v, want ErrUnauthorized", err)
	}
	messages, _, err := store.ListMessages(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("revoked in-flight upload created messages: %#v", messages)
	}
	entries, err := os.ReadDir(store.blobsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("revoked in-flight upload left blobs: %v", entries)
	}
}

func TestConcurrentCrossRevokeCannotRemoveEveryDevice(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, firstHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	_, secondHash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreateDevice(ctx, "First", firstHash)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateDevice(ctx, "Second", secondHash)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errorsSeen := make(chan error, 2)
	for _, pair := range [][2]string{{first.ID, second.ID}, {second.ID, first.ID}} {
		group.Add(1)
		go func(requester, target string) {
			defer group.Done()
			errorsSeen <- store.DeleteDevice(ctx, requester, target)
		}(pair[0], pair[1])
	}
	group.Wait()
	close(errorsSeen)
	succeeded := 0
	for err := range errorsSeen {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("cross revoke error = %v", err)
		}
	}
	devices, err := store.ListDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if succeeded != 1 || len(devices) != 1 {
		t.Fatalf("succeeded = %d, devices = %#v", succeeded, devices)
	}
}

func TestOpeningPreAuthDataAddsDevicesWithoutLosingMessages(t *testing.T) {
	dataDir := t.TempDir()
	ctx := context.Background()
	store, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	message, err := store.CreateText(ctx, "Legacy Web", "before auth")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "DROP TABLE devices"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	count, err := reopened.DeviceCount(ctx)
	if err != nil || count != 0 {
		t.Fatalf("device count = %d, error = %v", count, err)
	}
	messages, _, err := reopened.ListMessages(ctx, 0, 10)
	if err != nil || len(messages) != 1 || messages[0].ID != message.ID {
		t.Fatalf("messages = %#v, error = %v", messages, err)
	}
}

func TestAccessPasswordVerifierPersistsWithoutRawPassword(t *testing.T) {
	dataDir := t.TempDir()
	ctx := context.Background()
	store, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := store.AccessPassword(ctx); err != nil || value != nil {
		t.Fatalf("default access password = %#v, %v", value, err)
	}
	const password = "a raw password that must never persist"
	verifier, err := newAccessPasswordVerifier(password)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccessPassword(ctx, &verifier); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		contents, err := os.ReadFile(filepath.Join(dataDir, "ferry.db") + suffix)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if bytes.Contains(contents, []byte(password)) {
			t.Fatalf("raw password persisted in database file %q", suffix)
		}
	}
	reopened, err := OpenStore(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	loaded, err := reopened.AccessPassword(ctx)
	if err != nil || loaded == nil || !verifyAccessPassword(password, *loaded) {
		t.Fatalf("reopened verifier = %#v, %v", loaded, err)
	}
	if err := reopened.SetAccessPassword(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if loaded, err := reopened.AccessPassword(ctx); err != nil || loaded != nil {
		t.Fatalf("disabled verifier = %#v, %v", loaded, err)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

type failingReader struct{}

func (failingReader) Read(buffer []byte) (int, error) {
	copy(buffer, "partial")
	return len("partial"), errors.New("injected read failure")
}

type gatedReader struct {
	started chan struct{}
	release chan struct{}
	data    []byte
	once    sync.Once
}

func (reader *gatedReader) Read(buffer []byte) (int, error) {
	reader.once.Do(func() {
		close(reader.started)
		<-reader.release
	})
	if len(reader.data) == 0 {
		return 0, io.EOF
	}
	written := copy(buffer, reader.data)
	reader.data = reader.data[written:]
	return written, nil
}
