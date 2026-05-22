package doctor

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/devgrep/devgrep/internal/config"
	"github.com/devgrep/devgrep/internal/storage"
)

func TestRunAndPrint(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.Default()
	cfg.ConfigPath = filepath.Join(home, "config.yaml")
	cfg.DatabasePath = filepath.Join(home, "devgrep.db")
	cfg.IndexedPaths = []string{home}
	store, err := storage.Open(ctx, cfg.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	checks := Run(ctx, cfg, store)
	if len(checks) == 0 {
		t.Fatal("no checks")
	}
	var out bytes.Buffer
	Print(&out, checks, false)
	if !bytes.Contains(out.Bytes(), []byte("database")) {
		t.Fatalf("output = %s", out.String())
	}
}
