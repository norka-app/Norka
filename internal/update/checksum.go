package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

var (
	errChecksum = errors.New("checksum mismatch")
	errSize     = errors.New("size mismatch")
)

func parseDigest(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "sha256:") {
		lower = strings.TrimSpace(lower[len("sha256:"):])
	}
	if !isSHA256Hex(lower) {
		return ""
	}
	return lower
}

func isSHA256Hex(raw string) bool {
	if len(raw) != 64 {
		return false
	}
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

// parseChecksums reads GNU sha256sum lines and BSD "SHA256 (name) = hex" lines.
// Keys are the lowercase base name of the file.
func parseChecksums(text string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if name, sum, ok := parseBSDChecksum(line); ok {
			out[strings.ToLower(name)] = sum
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !isSHA256Hex(strings.ToLower(fields[0])) {
			continue
		}
		name := fields[len(fields)-1]
		name = strings.TrimPrefix(name, "*")
		name = filepath.Base(name)
		if name == "" || name == "." {
			continue
		}
		out[strings.ToLower(name)] = strings.ToLower(fields[0])
	}
	return out
}

func parseBSDChecksum(line string) (name, sum string, ok bool) {
	const prefix = "SHA256 ("
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(line, prefix)
	end := strings.Index(rest, ")")
	if end <= 0 {
		return "", "", false
	}
	name = filepath.Base(strings.TrimSpace(rest[:end]))
	after := strings.TrimSpace(rest[end+1:])
	after = strings.TrimPrefix(after, "=")
	after = strings.ToLower(strings.TrimSpace(after))
	if name == "" || !isSHA256Hex(after) {
		return "", "", false
	}
	return name, after, true
}

func digestForAsset(assetName, apiDigest, checksumsBody string) string {
	if parsed := parseDigest(apiDigest); parsed != "" {
		return parsed
	}
	if strings.TrimSpace(checksumsBody) == "" || strings.TrimSpace(assetName) == "" {
		return ""
	}
	sums := parseChecksums(checksumsBody)
	if sum, ok := sums[strings.ToLower(filepath.Base(assetName))]; ok {
		return sum
	}
	return ""
}

func verifyFile(path string, expectedSize int64, expectedSHA256 string) error {
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if !isSHA256Hex(expectedSHA256) {
		return errChecksum
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		return errSize
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return err
	}
	got := hex.EncodeToString(sum.Sum(nil))
	if got != expectedSHA256 {
		return fmt.Errorf("%w", errChecksum)
	}
	return nil
}
