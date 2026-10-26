package profile

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestIdentityChildProcess(t *testing.T) {
	if os.Getenv("TABLE_CARD_IDENTITY_CHILD") != "1" {
		return
	}
	id, err := Load("atomic:1781", "shared")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(id)
	if err := os.WriteFile(os.Getenv("TABLE_CARD_IDENTITY_RESULT"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityAtomicAcrossProcessesAndPreservesCorruption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const workers = 8
	var wg sync.WaitGroup
	outputs := make([][]byte, workers)
	errors := make([]error, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(executable, "-test.run=^TestIdentityChildProcess$")
			cmd.Env = append(os.Environ(), "TABLE_CARD_IDENTITY_CHILD=1", "TABLE_CARD_IDENTITY_RESULT="+filepath.Join(root, string(rune('a'+i))+".json"))
			outputs[i], errors[i] = cmd.CombinedOutput()
		}()
	}
	wg.Wait()
	id, err := Load("atomic:1781", "shared")
	if err != nil {
		t.Fatal(err)
	}
	for i := range workers {
		if errors[i] != nil {
			t.Fatalf("child %d: %v %s", i, errors[i], outputs[i])
		}
		data, err := os.ReadFile(filepath.Join(root, string(rune('a'+i))+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var other Identity
		if json.Unmarshal(data, &other) != nil || other != id {
			t.Fatal("processes received different identities")
		}
	}
	files, err := filepath.Glob(filepath.Join(root, "Table-Card", "players", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("identity files: %v %v", files, err)
	}
	temps, _ := filepath.Glob(filepath.Join(root, "Table-Card", "players", ".identity-*.tmp"))
	if len(temps) != 0 {
		t.Fatal("temporary identities were not cleaned up")
	}
	bad := []byte(`{"id":`)
	if err := os.WriteFile(files[0], bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("atomic:1781", "shared"); err == nil {
		t.Fatal("corruption was silently accepted")
	}
	after, _ := os.ReadFile(files[0])
	if string(after) != string(bad) {
		t.Fatal("corrupted identity was overwritten")
	}
}
