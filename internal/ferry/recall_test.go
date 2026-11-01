package ferry

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSharedRecallCursorAndFileAccess(t *testing.T) {
	handler, store := newRawTestApp(t, false)
	join := func() claimResponse {
		return decodeClaim(t, requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":"Same Mac","password":""}`))
	}
	owner, peer := join(), join()
	file, err := store.CreateFileForDevice(t.Context(), owner.Device, "original.txt", "text/plain", strings.NewReader("original bytes"))
	if err != nil {
		t.Fatal(err)
	}
	later, err := store.CreateTextForDevice(t.Context(), owner.Device, "keep this")
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/messages/" + file.ID
	assertStatus(t, authenticatedRequest(t, handler, http.MethodGet, file.File.DownloadURL, "", peer.Token), http.StatusOK)
	for range 2 {
		assertStatus(t, authenticatedRequest(t, handler, http.MethodDelete, path, "", peer.Token), http.StatusOK)
	}
	assertStatus(t, authenticatedRequest(t, handler, http.MethodGet, file.File.DownloadURL, "", peer.Token), http.StatusNotFound)
	if _, err := os.Stat(filepath.Join(store.blobsDir, file.ID+".blob")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("blob still exists: %v", err)
	}
	page := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/messages?after="+strconv.FormatInt(later.Sequence, 10)+"&after_retraction=0&limit=1", "", peer.Token)
	var payload struct {
		Messages []Message `json:"messages"`
		IDs      []string  `json:"retracted_ids"`
		Cursor   int64     `json:"next_retraction_cursor"`
	}
	decodeResponse(t, page, &payload)
	if len(payload.Messages) != 0 || len(payload.IDs) != 1 || payload.IDs[0] != file.ID || payload.Cursor == 0 {
		t.Fatalf("missed old recall: %#v", payload)
	}
	ids, next, err := store.ListRetractions(t.Context(), payload.Cursor, 1)
	if err != nil || len(ids) != 0 || next != payload.Cursor {
		t.Fatalf("retry duplicated recall: %v %d %v", ids, next, err)
	}
	assertStatus(t, authenticatedRequest(t, handler, http.MethodGet, "/api/v1/messages?after_retraction=-1", "", owner.Token), http.StatusBadRequest)
	if err := store.DeleteDevice(t.Context(), peer.Device.ID, owner.Device.ID); err != nil {
		t.Fatal(err)
	}
	// Bypass middleware to reproduce a request authenticated before revocation.
	if err := store.RetractMessage(t.Context(), owner.Device.ID, later.ID); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("stale requester: %v", err)
	}
}

func TestRecallPaginationAndRestartCleanup(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	_, hash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.CreateDevice(t.Context(), "Mac", hash)
	if err != nil {
		t.Fatal(err)
	}
	var expected []string
	for range 3 {
		message, err := store.CreateTextForDevice(t.Context(), owner, "recall me")
		if err != nil {
			t.Fatal(err)
		}
		expected = append(expected, message.ID)
		if err := store.RetractMessage(t.Context(), owner.ID, message.ID); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate a stop after intent is committed but before unlink/finalise.
	pending, err := store.CreateFileForDevice(t.Context(), owner, "pending.txt", "text/plain", strings.NewReader("interrupted unlink"))
	if err != nil {
		t.Fatal(err)
	}
	expected = append(expected, pending.ID)
	orphan := filepath.Join(store.blobsDir, pending.ID+".blob")
	if _, err := store.db.Exec(`INSERT INTO retractions (message_id) VALUES (?)`, pending.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted unlink not completed: %v", err)
	}
	cursor := int64(0)
	for _, id := range expected {
		ids, next, err := store.ListRetractions(t.Context(), cursor, 1)
		if err != nil || len(ids) != 1 || ids[0] != id || next <= cursor {
			t.Fatalf("lost recall: %v %d %v", ids, next, err)
		}
		cursor = next
	}
	messages, _, err := store.ListMessages(t.Context(), 0, 100)
	if err != nil || len(messages) != 0 {
		t.Fatalf("recalled content survived restart: %v %v", messages, err)
	}
}

func TestFailedRecallRemainsRetryableWithoutSkippingItsSequence(t *testing.T) {
	store := openTestStore(t)
	_, hash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.CreateDevice(t.Context(), "Mac", hash)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreateTextForDevice(t.Context(), owner, "retry me")
	if err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(store.blobsDir, first.ID+".blob")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "block"), []byte("block unlink"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.RetractMessage(t.Context(), owner.ID, first.ID); err == nil {
		t.Fatal("expected unlink failure")
	}
	messages, _, err := store.ListMessages(t.Context(), 0, 10)
	if err != nil || len(messages) != 1 {
		t.Fatalf("failure lost retry control: %v %v", messages, err)
	}
	ids, _, err := store.ListRetractions(t.Context(), 0, 10)
	if err != nil || len(ids) != 0 {
		t.Fatalf("published failed recall: %v %v", ids, err)
	}
	second, err := store.CreateTextForDevice(t.Context(), owner, "finish first")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RetractMessage(t.Context(), owner.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	_, cursor, err := store.ListRetractions(t.Context(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(blocked); err != nil {
		t.Fatal(err)
	}
	if err := store.RetractMessage(t.Context(), owner.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	ids, next, err := store.ListRetractions(t.Context(), cursor, 10)
	if err != nil || len(ids) != 1 || ids[0] != first.ID || next <= cursor {
		t.Fatalf("retry fell behind cursor: %v %d %v", ids, next, err)
	}
}

func TestSharedRecallAlsoRemovesLegacyMessages(t *testing.T) {
	store := openTestStore(t)
	_, hash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.CreateDevice(t.Context(), "Mac", hash)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.CreateText(t.Context(), "Legacy", "old content")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RetractMessage(t.Context(), device.ID, legacy.ID); err != nil {
		t.Fatal(err)
	}
	messages, _, err := store.ListMessages(t.Context(), 0, 10)
	if err != nil || len(messages) != 0 {
		t.Fatalf("legacy recall failed: %v %v", messages, err)
	}
}
