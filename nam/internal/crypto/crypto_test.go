package crypto

import (
	"bytes"
	"testing"
)

func TestSealOpen(t *testing.T) {
	b, err := New("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	pt := []byte("sk-plaintext-test")
	blob, err := b.Seal(pt)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, pt) {
		t.Fatal("ciphertext contains plaintext")
	}
	got, err := b.Open(blob)
	if err != nil || !bytes.Equal(got, pt) {
		t.Fatalf("open: %v %q", err, got)
	}
	other, _ := New("other")
	if _, err := other.Open(blob); err == nil {
		t.Fatal("wrong key must fail")
	}
	if _, err := New(""); err != ErrNoSecret {
		t.Fatal("empty secret must fail")
	}
}
