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
	"github.com/zalando/go-keyring"
)

const keyringService = "ship-age-key"

// KeyNotFound is returned when no age key exists. The CLI should prompt the user.
var KeyNotFound = fmt.Errorf("no age key found — run 'ship keys generate'")

// KeyStore tracks where the key was found.
type KeyStore string

const (
	KeyStoreEnv      KeyStore = "env"
	KeyStoreKeychain KeyStore = "keychain"
	KeyStoreFile     KeyStore = "file"
)

func privateKeyPath() string {
	return filepath.Join(config.StateDir(), "age-key.txt")
}

// DetectKeyStore returns where the key is stored, or KeyNotFound.
func DetectKeyStore() (KeyStore, error) {
	if os.Getenv("SHIP_AGE_KEY") != "" {
		return KeyStoreEnv, nil
	}
	if key, err := keyring.Get(keyringService, "private"); err == nil && key != "" {
		return KeyStoreKeychain, nil
	}
	if _, err := os.Stat(privateKeyPath()); err == nil {
		return KeyStoreFile, nil
	}
	return "", KeyNotFound
}

// Generate creates a new keypair and stores it in the given store.
func Generate(store KeyStore) (*age.X25519Identity, error) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("generating age key: %w", err)
	}

	switch store {
	case KeyStoreFile:
		os.MkdirAll(config.StateDir(), 0700)
		os.WriteFile(privateKeyPath(), []byte(id.String()), 0600)
	case KeyStoreKeychain:
		keyring.Set(keyringService, "private", id.String())
	case KeyStoreEnv:
		// Don't store — user sets SHIP_AGE_KEY themselves
	default:
		return nil, fmt.Errorf("unknown key store: %s", store)
	}

	return id, nil
}

func EnsureKeys() (*age.X25519Identity, error) {
	// 1. Env var
	if envKey := os.Getenv("SHIP_AGE_KEY"); envKey != "" {
		return parseIdentity(envKey)
	}
	// 2. Keychain
	if key, err := keyring.Get(keyringService, "private"); err == nil && key != "" {
		if id, err := parseIdentity(key); err == nil {
			return id, nil
		}
	}
	// 3. File
	if data, err := os.ReadFile(privateKeyPath()); err == nil {
		if id, err := parseIdentity(strings.TrimSpace(string(data))); err == nil {
			return id, nil
		}
	}
	return nil, KeyNotFound
}

func parseIdentity(key string) (*age.X25519Identity, error) {
	ids, err := age.ParseIdentities(strings.NewReader(strings.TrimSpace(key)))
	if err != nil || len(ids) == 0 {
		return nil, fmt.Errorf("invalid age key")
	}
	xid, ok := ids[0].(*age.X25519Identity)
	if !ok {
		return nil, fmt.Errorf("unexpected key type")
	}
	return xid, nil
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
