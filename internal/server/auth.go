package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

func authorized(r *http.Request, id string) bool {
	token := r.Header.Get("X-Player-Token")
	if len(token) < 32 || id == "" {
		return false
	}
	sum := sha256.Sum256([]byte(token))
	expected := hex.EncodeToString(sum[:16])
	return subtle.ConstantTimeCompare([]byte(id), []byte(expected)) == 1
}
func authorize(w http.ResponseWriter, r *http.Request, id string) bool {
	if authorized(r, id) {
		return true
	}
	writeError(w, 403, "玩家身份验证失败，请使用当前终端客户端")
	return false
}
