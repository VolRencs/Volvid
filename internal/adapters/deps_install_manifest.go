package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func scanChecksumManifest(manifest string, match func(asset string) bool) (string, string, error) {
	for _, line := range strings.Split(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		asset := fields[len(fields)-1]
		if !match(asset) {
			continue
		}
		checksum, err := normalizeSHA256(fields[0])
		if err != nil {
			return "", "", fmt.Errorf("normalize checksum: %w", err)
		}
		return asset, checksum, nil
	}
	return "", "", nil
}
func nodeAssetFromManifest(manifest, suffix string) (string, string, error) {
	name, checksum, err := scanChecksumManifest(manifest, func(asset string) bool {
		return strings.HasSuffix(asset, suffix)
	})
	if err != nil {
		return "", "", fmt.Errorf("node checksum for %s: %w", name, err)
	}
	if name == "" {
		return "", "", fmt.Errorf("node asset with suffix %s not found", suffix)
	}
	name = filepath.Base(strings.TrimSpace(name))
	if !validAssetFilename(name) {
		return "", "", fmt.Errorf("node asset filename is invalid: %q", name)
	}
	return name, checksum, nil
}
func validAssetFilename(name string) bool {
	if name == "" || name != filepath.Base(name) {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == '+':
		default:
			return false
		}
	}
	return true
}
func checksumFromManifest(manifest, name string) (string, error) {
	target := strings.TrimSpace(name)
	found, checksum, err := scanChecksumManifest(manifest, func(asset string) bool {
		return asset == target
	})
	if err != nil {
		return "", fmt.Errorf("scan checksum manifest: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("asset %s not found", name)
	}
	return checksum, nil
}
func nodeAssetSuffix() (string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", fmt.Errorf("detect platform: %w", err)
	}
	if platform.NodeAssetSuffix == "" {
		return "", fmt.Errorf("node asset suffix is empty")
	}
	return platform.NodeAssetSuffix, nil
}
func normalizeSHA256(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != sha256.Size*2 {
		return "", fmt.Errorf("expected %d hex chars", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("decode hex string: %w", err)
	}
	return value, nil
}
func verifyFileSHA256(path, expected string) error {
	expected, err := normalizeSHA256(expected)
	if err != nil {
		return fmt.Errorf("normalize expected checksum: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return fmt.Errorf("read file for checksum: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("sha256 mismatch for %s", filepath.Base(path))
	}
	return nil
}
