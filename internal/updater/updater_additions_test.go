package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// ── validateDownloadURL ───────────────────────────────────────────────────

func TestValidateDownloadURL_AllowedGithubRelease(t *testing.T) {
	validURLs := []string{
		"https://github.com/ardepa710/sentineledge-agent/releases/download/v2026.03.10-1/sentineledge-agent.exe",
		"https://github.com/ardepa710/sentineledge-agent/releases/download/v2026.04.01-2/sentineledge-agent.exe",
		"https://objects.githubusercontent.com/github-production-release-asset-123/sentineledge-agent.exe",
	}
	for _, u := range validURLs {
		if err := validateDownloadURL(u); err != nil {
			t.Errorf("expected URL %q to be allowed, got: %v", u, err)
		}
	}
}

func TestValidateDownloadURL_RejectedDomains(t *testing.T) {
	rejectedURLs := []string{
		"https://evil.com/sentineledge-agent.exe",
		"http://github.com/ardepa710/sentineledge-agent/releases/download/v1/agent.exe",
		"https://github.com.evil.io/sentineledge-agent.exe",
		"https://othergithub.com/ardepa710/sentineledge-agent/",
		"ftp://objects.githubusercontent.com/agent.exe",
		"",
		"javascript:alert(1)",
	}
	for _, u := range rejectedURLs {
		if err := validateDownloadURL(u); err == nil {
			t.Errorf("expected URL %q to be rejected, but it was allowed", u)
		}
	}
}

func TestValidateDownloadURL_HTTPNotAllowed(t *testing.T) {
	// Must be HTTPS — plain HTTP must be rejected even for allowed domains
	url := "http://github.com/ardepa710/sentineledge-agent/releases/download/v1/agent.exe"
	if err := validateDownloadURL(url); err == nil {
		t.Error("HTTP (non-TLS) URL must be rejected even for allowed domains")
	}
}

// ── verifyHash ────────────────────────────────────────────────────────────

func TestVerifyHash_CorrectHash(t *testing.T) {
	content := []byte("sentineledge test binary content")
	h := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(h[:])

	tmpFile, err := os.CreateTemp("", "hash_test_*.bin")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	if err := verifyHash(tmpFile.Name(), expectedHash); err != nil {
		t.Errorf("correct hash should pass verification, got: %v", err)
	}
}

func TestVerifyHash_WrongHashFails(t *testing.T) {
	content := []byte("sentineledge test binary content")

	tmpFile, err := os.CreateTemp("", "hash_test_*.bin")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	wrongHash := "aabbccdd00112233aabbccdd00112233aabbccdd00112233aabbccdd00112233"
	if err := verifyHash(tmpFile.Name(), wrongHash); err == nil {
		t.Error("wrong hash should fail verification")
	}
}

func TestVerifyHash_NonExistentFileReturnsError(t *testing.T) {
	err := verifyHash("/nonexistent/path/file.exe", "aabbccdd00112233aabbccdd00112233aabbccdd00112233aabbccdd00112233")
	if err == nil {
		t.Error("verifying hash of non-existent file must return an error")
	}
}

// ── isNewer edge cases ─────────────────────────────────────────────────────

func TestIsNewer_MalformedVersionReturnsFalse(t *testing.T) {
	cases := []struct {
		candidate string
		current   string
	}{
		{"invalid", "v2026.03.10-1"},
		{"v2026.03.10-1", "invalid"},
		{"", "v2026.03.10-1"},
		{"v2026.03.10-1", ""},
		{"v2026.03-1", "v2026.03.10-1"},    // missing day
		{"v2026.03.10", "v2026.03.10-1"},   // missing build
		{"vNaN.03.10-1", "v2026.03.10-1"},  // non-numeric year
	}
	for _, tc := range cases {
		if isNewer(tc.candidate, tc.current) {
			t.Errorf("isNewer(%q, %q) = true, want false (malformed input)", tc.candidate, tc.current)
		}
	}
}

func TestIsNewer_MonthComparison(t *testing.T) {
	if !isNewer("v2026.04.01-1", "v2026.03.31-1") {
		t.Error("April 1 should be newer than March 31 of same year")
	}
}
