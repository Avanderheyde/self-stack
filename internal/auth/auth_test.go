package auth

import "testing"

func TestIssueAndVerify(t *testing.T) {
	dir := t.TempDir()
	a, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	token, err := a.IssueToken("device-123")
	if err != nil {
		t.Fatal(err)
	}
	deviceID, ok := a.VerifyToken(token)
	if !ok {
		t.Fatal("token verification failed")
	}
	if deviceID != "device-123" {
		t.Fatalf("expected device-123, got %s", deviceID)
	}
}

func TestVerifyToken_Invalid(t *testing.T) {
	dir := t.TempDir()
	a, _ := New(dir)
	_, ok := a.VerifyToken("garbage.token")
	if ok {
		t.Fatal("expected invalid token to fail verification")
	}
}

func TestKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	a1, _ := New(dir)
	token, _ := a1.IssueToken("device-1")

	a2, _ := New(dir)
	deviceID, ok := a2.VerifyToken(token)
	if !ok || deviceID != "device-1" {
		t.Fatal("token from first instance should verify with second instance")
	}
}

func TestTokenHash(t *testing.T) {
	h := TokenHash("test-token")
	if len(h) != 64 {
		t.Fatalf("expected 64 char hex hash, got %d chars", len(h))
	}
}
