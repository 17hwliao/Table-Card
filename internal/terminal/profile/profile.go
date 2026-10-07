package profile

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	if id, err := readIdentity(path); err == nil {
		return id, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Identity{}, err
	}
	var id Identity
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
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return readIdentity(path)
	}
	if err != nil {
		return Identity{}, err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return Identity{}, err
	}
	return id, nil
}

func readIdentity(path string) (Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Identity{}, err
	}
	var id Identity
	if json.Unmarshal(data, &id) != nil {
		return Identity{}, fmt.Errorf("身份文件无法解析，已保留原文件：%s；请备份后恢复该文件或使用另一个昵称", path)
	}
	secret, decodeErr := hex.DecodeString(id.Token)
	hash := sha256.Sum256([]byte(id.Token))
	if decodeErr != nil || len(secret) != 32 || id.ID != hex.EncodeToString(hash[:16]) {
		return Identity{}, fmt.Errorf("身份文件损坏，已保留原文件：%s；请备份后恢复该文件或使用另一个昵称", path)
	}
	return id, nil
}
