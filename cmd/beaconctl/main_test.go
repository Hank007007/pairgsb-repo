package main

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beaconproof/internal/tsa"
)

// setupApp builds an App under a temporary home directory together with a
// running in-process authority whose public key the App will use.
func setupApp(t *testing.T) (*App, *tsa.Authority, *httptest.Server, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()

	auth, err := tsa.NewAuthority()
	if err != nil {
		t.Fatalf("authority: %v", err)
	}
	srv := httptest.NewServer(auth.Handler())
	t.Cleanup(srv.Close)

	layout := DefaultLayout(root)
	if err := auth.WritePublicKey(layout.PubPath); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	var out bytes.Buffer
	app := NewApp(layout, &out, &out)
	app.BaseURL = srv.URL
	return app, auth, srv, &out
}

func TestVerifyPassesAfterSequentialAppendAndAnchor(t *testing.T) {
	app, _, _, out := setupApp(t)

	for i := 0; i < 200; i++ {
		code := app.Run([]string{
			"append",
			"--actor", "alice",
			"--action", "read",
			"--body", fmt.Sprintf("doc-%03d", i),
		})
		if code != 0 {
			t.Fatalf("append %d exited %d: %s", i, code, out.String())
		}
	}

	out.Reset()
	if code := app.Run([]string{"anchor", "--n", "200"}); code != 0 {
		t.Fatalf("anchor exited %d: %s", code, out.String())
	}

	out.Reset()
	if code := app.Run([]string{"verify"}); code != 0 {
		t.Fatalf("verify exited %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "verify: OK") {
		t.Fatalf("verify output does not report success: %s", out.String())
	}
}

func TestVerifyOnEmptyLog(t *testing.T) {
	app, _, _, out := setupApp(t)

	if code := app.Run([]string{"verify"}); code != 0 {
		t.Fatalf("verify on empty log exited %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "0 records") {
		t.Fatalf("verify did not count an empty log: %s", out.String())
	}
}

func TestAppendRequiresActorAndAction(t *testing.T) {
	app, _, _, _ := setupApp(t)
	if code := app.Run([]string{"append", "--actor", "alice"}); code == 0 {
		t.Fatal("append without an action succeeded")
	}
}

func TestLogFileIsCreatedUnderHome(t *testing.T) {
	app, _, _, _ := setupApp(t)
	if code := app.Run([]string{"append", "--actor", "a", "--action", "b"}); code != 0 {
		t.Fatalf("append exited %d", code)
	}
	if _, err := os.Stat(app.Layout.LogPath); err != nil {
		t.Fatalf("log file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(app.Layout.ReceiptDir)); err != nil {
		t.Fatalf("receipt directory missing: %v", err)
	}
}
