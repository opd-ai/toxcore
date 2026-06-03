package crypto

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// forbiddenLogPatterns lists regular expressions that match log statements that
// could expose full private key material. These patterns are enforced at test
// time as a lightweight CI substitute: if any source file in the crypto package
// matches, the test fails.
//
// Rationale:
//   - Private keys must never appear in log output, even at debug level.
//   - Only safe patterns (key prefix, hash preview, size) are permitted.
//   - `SecureFieldHash` in this package is the approved safe alternative.
var forbiddenLogPatterns = []*regexp.Regexp{
	// Logging a full 32-byte key variable without slicing to a short prefix.
	// Matches patterns like: log.Printf("%x", privateKey), logrus.WithField("key", privKey)
	regexp.MustCompile(`(?i)(log|logrus).*("%x"|"%v"|"%s").*\bprivate\b`),
	regexp.MustCompile(`(?i)WithField\s*\(\s*"(private_key|secret_key|private|sk|secret)"\s*,\s*\w+\s*\)`),
	// Matches direct sprint of a key variable into a log field.
	regexp.MustCompile(`(?i)fmt\.Sprintf\s*\(\s*"%x"\s*,\s*\w*(private|secret|sk)\w*\s*\)`),
}

// TestNoRawKeyMaterialInCryptoPackageLogs scans every .go source file in the
// crypto package directory and fails if any line matches a forbidden log pattern.
// This enforces the safe-logging guideline from logging.go at test time.
func TestNoRawKeyMaterialInCryptoPackageLogs(t *testing.T) {
	// Find the directory of this test file (crypto/).
	dir, err := findCryptoPackageDir()
	if err != nil {
		t.Skipf("could not locate crypto package directory: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		checkFileForForbiddenPatterns(t, path)
	}
}

// TestSecureFieldHashNeverExposesRawKey verifies that SecureFieldHash returns a
// hashed preview, not raw key bytes, so it is safe to pass to log output.
func TestSecureFieldHashNeverExposesRawKey(t *testing.T) {
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}
	rawHex := encodeHex(rawKey)

	fields := SecureFieldHash(rawKey, "key")

	preview, ok := fields["key_preview"]
	if !ok {
		t.Fatal("SecureFieldHash did not return key_preview field")
	}
	previewStr, ok := preview.(string)
	if !ok {
		t.Fatal("key_preview field is not a string")
	}

	// The preview must NOT contain the raw key hex.
	if strings.Contains(previewStr, rawHex) {
		t.Errorf("SecureFieldHash preview contains raw key material: %s", previewStr)
	}

	// The preview must contain "..." indicating truncation.
	if !strings.Contains(previewStr, "...") {
		t.Errorf("SecureFieldHash preview does not contain truncation indicator: %s", previewStr)
	}

	// Size field must be present and correct.
	size, ok := fields["key_size"]
	if !ok {
		t.Fatal("SecureFieldHash did not return key_size field")
	}
	if size != len(rawKey) {
		t.Errorf("key_size = %v, want %d", size, len(rawKey))
	}
}

// checkFileForForbiddenPatterns scans a file line-by-line against forbidden patterns.
func checkFileForForbiddenPatterns(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("open %s: %v", path, err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// Skip comments.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, pat := range forbiddenLogPatterns {
			if pat.MatchString(line) {
				t.Errorf("%s:%d: forbidden log pattern detected (%s):\n  %s",
					filepath.Base(path), lineNum, pat, strings.TrimSpace(line))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Errorf("scan %s: %v", path, err)
	}
}

// findCryptoPackageDir returns the directory containing this test file.
// It walks upward from the working directory looking for "crypto/".
func findCryptoPackageDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return wd, nil
}

// encodeHex returns a lowercase hex string for b (used in test assertions only).
func encodeHex(b []byte) string {
	const hexChars = "0123456789abcdef"
	buf := make([]byte, len(b)*2)
	for i, c := range b {
		buf[i*2] = hexChars[c>>4]
		buf[i*2+1] = hexChars[c&0xF]
	}
	return string(buf)
}
