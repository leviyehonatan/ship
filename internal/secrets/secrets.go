package secrets

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
	"github.com/leviyehonatan/ship/internal/config"
)

func privateKeyPath() string {
	return filepath.Join(config.StateDir(), "age-key.txt")
}

func EnsureKeys() (*age.X25519Identity, error) {
	if _, err := os.Stat(privateKeyPath()); err == nil {
		data, err := os.ReadFile(privateKeyPath())
		if err != nil {
			return nil, err
		}
		ids, err := age.ParseIdentities(strings.NewReader(strings.TrimSpace(string(data))))
		if err != nil || len(ids) == 0 {
			return nil, fmt.Errorf("parsing key: %w", err)
		}
		xid, ok := ids[0].(*age.X25519Identity)
		if !ok {
			return nil, fmt.Errorf("unexpected key type")
		}
		return xid, nil
	}

	id, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}

	os.MkdirAll(config.StateDir(), 0700)
	os.WriteFile(privateKeyPath(), []byte(id.String()), 0600)

	return id, nil
}

// --- direct operations on .env.encrypted ---

func Set(encPath, key, value string) error {
	m, err := readEncrypted(encPath)
	if err != nil {
		return err
	}
	m[key] = value
	return writeEncrypted(encPath, m)
}

func Unset(encPath, key string) error {
	m, err := readEncrypted(encPath)
	if err != nil {
		return err
	}
	delete(m, key)
	return writeEncrypted(encPath, m)
}

func List(encPath string) ([]string, error) {
	m, err := readEncrypted(encPath)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// DecryptToEnv writes the plaintext .env file from .env.encrypted.
func DecryptToEnv(encPath, envPath string) error {
	if envPath == "" {
		envPath = strings.TrimSuffix(encPath, ".encrypted")
	}
	m, err := readEncrypted(encPath)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	for k, v := range m {
		fmt.Fprintf(&buf, "%s=%s\n", k, v)
	}
	return os.WriteFile(envPath, buf.Bytes(), 0644)
}

// EncryptFile reads a plaintext .env and writes the encrypted version.
func EncryptFile(plainPath string) error {
	data, err := os.ReadFile(plainPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", plainPath, err)
	}
	m := parseEnv(data)
	encPath := plainPath + ".encrypted"
	return writeEncrypted(encPath, m)
}

// ReadAll returns all key-value pairs from an encrypted file.
func ReadAll(encPath string) (map[string]string, error) {
	return readEncrypted(encPath)
}

// --- internal ---

func readEncrypted(path string) (map[string]string, error) {
	id, err := EnsureKeys()
	if err != nil {
		return nil, err
	}

	encrypted, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	r, err := age.Decrypt(bytes.NewReader(encrypted), id)
	if err != nil {
		return nil, fmt.Errorf("decrypting %s: %w", path, err)
	}

	var decrypted bytes.Buffer
	if _, err := decrypted.ReadFrom(r); err != nil {
		return nil, err
	}

	return parseEnv(decrypted.Bytes()), nil
}

func writeEncrypted(path string, m map[string]string) error {
	id, err := EnsureKeys()
	if err != nil {
		return err
	}

	var plain bytes.Buffer
	for k, v := range m {
		fmt.Fprintf(&plain, "%s=%s\n", k, v)
	}

	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, id.Recipient())
	if err != nil {
		return err
	}
	w.Write(plain.Bytes())
	w.Close()

	return os.WriteFile(path, encrypted.Bytes(), 0644)
}

func parseEnv(data []byte) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			m[k] = v
		}
	}
	return m
}
