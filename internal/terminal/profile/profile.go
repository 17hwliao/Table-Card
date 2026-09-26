package profile

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Identity struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

func Load(server, name string) (Identity, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return Identity{}, err
	}
	key := sha256.Sum256([]byte(server + "\n" + name))
	dir := filepath.Join(root, "Table-Card", "players")
	path := filepath.Join(dir, hex.EncodeToString(key[:16])+".json")
	var id Identity
	if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &id) == nil && id.Token != "" {
		return id, nil
	}
	var secret [32]byte
	if _, err = rand.Read(secret[:]); err != nil {
		return id, err
	}
	id.Token = hex.EncodeToString(secret[:])
	hash := sha256.Sum256([]byte(id.Token))
	id.ID = hex.EncodeToString(hash[:16])
	if err = os.MkdirAll(dir, 0700); err != nil {
		return id, err
	}
	data, _ := json.Marshal(id)
	err = os.WriteFile(path, data, 0600)
	return id, err
}
