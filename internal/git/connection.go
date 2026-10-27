package git

import (
	"encoding/json"
	"errors"
	"github.com/buchin/cuan/internal/crypto"
	"os"
	"path/filepath"
)

// Connection is persisted privately; Token is never serialized in plaintext.
type Connection struct {
	Account        Account `json:"account"`
	EncryptedToken string  `json:"encrypted_token"`
}

func SaveConnection(path, secret, token string, account Account) error {
	if token == "" || account.Login == "" {
		return errors.New("GitHub connection is incomplete")
	}
	encrypted, err := crypto.Encrypt(token, secret)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(Connection{account, encrypted})
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".github-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	return err
}
func LoadConnection(path, secret string) (Account, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Account{}, "", err
	}
	var saved Connection
	if json.Unmarshal(raw, &saved) != nil || saved.Account.Login == "" || saved.EncryptedToken == "" {
		return Account{}, "", errors.New("Saved GitHub connection is invalid")
	}
	token, err := crypto.Decrypt(saved.EncryptedToken, secret)
	if err != nil {
		return Account{}, "", errors.New("Saved GitHub access could not be read")
	}
	return saved.Account, token, nil
}
