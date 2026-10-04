package secret

import (
	"bytes"
	"errors"
	"testing"
)

func TestSealOpen(t *testing.T) {
	box := FromPassphrase("test")
	sealed, err := box.Seal([]byte("001234567890"), []byte("property-a"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("001234567890")) {
		t.Fatal("ciphertext contains the plaintext")
	}
	plain, err := box.Open(sealed, []byte("property-a"))
	if err != nil || string(plain) != "001234567890" {
		t.Fatalf("open = %q, %v", plain, err)
	}
	if _, err := box.Open(sealed, []byte("property-b")); !errors.Is(err, ErrOpen) {
		t.Errorf("open with another record = %v, want ErrOpen", err)
	}
	if _, err := FromPassphrase("other").Open(sealed, []byte("property-a")); !errors.Is(err, ErrOpen) {
		t.Errorf("open with another key = %v, want ErrOpen", err)
	}
	if _, err := New([]byte("short")); err == nil {
		t.Error("short key accepted")
	}
}
