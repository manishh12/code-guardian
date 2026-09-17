package githubapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignatureAcceptsValidHMAC(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	secret := "change-me"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	header := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifySignature(secret, body, header) {
		t.Fatal("expected valid signature")
	}
}

func TestVerifySignatureRejectsTamper(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	secret := "change-me"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	header := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if VerifySignature(secret, []byte(`{"action":"closed"}`), header) {
		t.Fatal("tampered body should fail")
	}
	if VerifySignature(secret, body, "sha256=deadbeef") {
		t.Fatal("wrong digest should fail")
	}
	if VerifySignature("", body, header) {
		t.Fatal("empty secret should fail")
	}
}

func TestDemoPullRequestHasUnsafeDiff(t *testing.T) {
	pr := DemoPullRequest()
	if pr.PRNumber != 1 || pr.Diff == "" {
		t.Fatal("demo PR should include a diff")
	}
}
