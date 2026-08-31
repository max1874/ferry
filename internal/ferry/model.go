package ferry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxTextBytes   = 64 << 10
	MaxSenderBytes = 64
	MaxFileBytes   = 64 << 20
	MaxFileName    = 255
)

var (
	ErrInvalid      = errors.New("invalid input")
	ErrTooLarge     = errors.New("input too large")
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
)

type Kind string

const (
	KindText Kind = "text"
	KindFile Kind = "file"
)

type DeviceKind string

const (
	DeviceKindIPhone  DeviceKind = "iphone"
	DeviceKindIPad    DeviceKind = "ipad"
	DeviceKindMac     DeviceKind = "mac"
	DeviceKindAndroid DeviceKind = "android"
	DeviceKindWindows DeviceKind = "windows"
	DeviceKindBrowser DeviceKind = "browser"
)

type FileInfo struct {
	Name        string `json:"name"`
	MediaType   string `json:"media_type"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

type Message struct {
	ID              string     `json:"id"`
	Sequence        int64      `json:"sequence"`
	Kind            Kind       `json:"kind"`
	SenderName      string     `json:"sender_name"`
	SenderKind      DeviceKind `json:"sender_kind"`
	SenderDeviceID  string     `json:"-"`
	IsCurrentDevice bool       `json:"is_current_device"`
	CreatedAt       string     `json:"created_at"`
	Text            *string    `json:"text,omitempty"`
	File            *FileInfo  `json:"file,omitempty"`
}

type Device struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      DeviceKind `json:"kind"`
	CreatedAt string     `json:"created_at"`
}

func normalizeDeviceKind(value string, deviceName string) (DeviceKind, error) {
	if value == "" {
		return inferDeviceKind(deviceName), nil
	}
	kind := DeviceKind(value)
	if !validDeviceKind(kind) {
		return "", fmt.Errorf("%w: unknown device kind %q", ErrInvalid, value)
	}
	return kind, nil
}

func validDeviceKind(kind DeviceKind) bool {
	switch kind {
	case DeviceKindIPhone, DeviceKindIPad, DeviceKindMac, DeviceKindAndroid, DeviceKindWindows, DeviceKindBrowser:
		return true
	default:
		return false
	}
}

func inferDeviceKind(deviceName string) DeviceKind {
	name := strings.ToLower(deviceName)
	switch {
	case strings.Contains(name, "ipad"):
		return DeviceKindIPad
	case strings.Contains(name, "iphone"), strings.Contains(name, "ipod"):
		return DeviceKindIPhone
	case strings.Contains(name, "android"):
		return DeviceKindAndroid
	case strings.Contains(name, "mac"):
		return DeviceKindMac
	case strings.Contains(name, "windows"):
		return DeviceKindWindows
	default:
		return DeviceKindBrowser
	}
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func normalizeSenderName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: sender_name is required", ErrInvalid)
	}
	if len(value) > MaxSenderBytes {
		return "", fmt.Errorf("%w: sender_name exceeds %d UTF-8 bytes", ErrTooLarge, MaxSenderBytes)
	}
	if !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return "", fmt.Errorf("%w: sender_name contains invalid characters", ErrInvalid)
	}
	return value, nil
}

func validateText(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: text must contain a non-whitespace character", ErrInvalid)
	}
	if len(value) > MaxTextBytes {
		return fmt.Errorf("%w: text exceeds %d UTF-8 bytes", ErrTooLarge, MaxTextBytes)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: text is not valid UTF-8", ErrInvalid)
	}
	return nil
}

func normalizeFileName(value string) (string, error) {
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Base(value)
	if value == "." || value == ".." || value == "/" || value == "" {
		return "", fmt.Errorf("%w: file name is required", ErrInvalid)
	}
	if len(value) > MaxFileName {
		return "", fmt.Errorf("%w: file name exceeds %d UTF-8 bytes", ErrTooLarge, MaxFileName)
	}
	if !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return "", fmt.Errorf("%w: file name contains invalid characters", ErrInvalid)
	}
	return value, nil
}

func normalizeMediaType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" {
		return "application/octet-stream"
	}
	return mediaType
}

func validID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}
