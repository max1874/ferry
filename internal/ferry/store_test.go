package ferry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

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
