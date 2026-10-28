package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/selfupdate"
)

// fakeRelease serves one release and its tarball, shaped like the GitHub API.
func fakeRelease(t *testing.T, tag string, payload []byte) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "lazyswap", Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := buf.Bytes()
	digest := sha256.Sum256(archive)
	asset := "lazyswap-" + mustTarget(t) + ".tar.gz"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		body := fmt.Sprintf(
			`{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":"%s/dl/a"},{"name":%q,"browser_download_url":"%s/dl/s"}]}`,
			tag, asset, base, asset+".sha256", base)
		switch r.URL.Path {
		case "/repos/o/p/releases/latest", "/repos/o/p/releases/tags/" + tag:
			_, _ = w.Write([]byte(body))
		case "/dl/a":
			_, _ = w.Write(archive)
		case "/dl/s":
			_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(digest[:]), asset)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// captureStderr collects what fn writes to stderr; die() reports there.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()
	fn()
	os.Stderr = old
	_ = w.Close()
	return <-done
}

func mustTarget(t *testing.T) string {
	t.Helper()
	target, err := selfupdate.Target(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("no release build for this platform: %v", err)
	}
	return target
}

// stubUpdate points the command at a local server and a throwaway binary.
func stubUpdate(t *testing.T, tag, installed string, payload []byte) string {
	t.Helper()
	base := fakeRelease(t, tag, payload)

	oldUpdater, oldExe, oldVersion := newUpdater, exePath, version
	t.Cleanup(func() { newUpdater, exePath, version = oldUpdater, oldExe, oldVersion })

	exe := filepath.Join(t.TempDir(), "lazyswap")
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	newUpdater = func() selfupdate.Updater {
		return selfupdate.Updater{APIBase: base, Repo: "o/p", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	}
	exePath = func() (string, error) { return exe, nil }
	version = installed
	return exe
}

func TestUpdateCheckInstallsNothing(t *testing.T) {
	exe := stubUpdate(t, "v9.9.9", "v1.0.0", []byte("new"))
	out, code := captureStdout(t, func() int { return runUpdate([]string{"--check"}) })
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, want := range []string{"v1.0.0", "v9.9.9"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q should mention %q", out, want)
		}
	}
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Fatalf("--check replaced the binary with %q", got)
	}
}

func TestUpdateSaysWhenAlreadyCurrent(t *testing.T) {
	exe := stubUpdate(t, "v9.9.9", "v9.9.9", []byte("new"))
	out, code := captureStdout(t, func() int { return runUpdate(nil) })
	if code != 0 || !strings.Contains(out, "already up to date") {
		t.Fatalf("exit %d, out %q", code, out)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Fatalf("binary replaced when already current: %q", got)
	}
}

func TestUpdateInstallsTheNewRelease(t *testing.T) {
	exe := stubUpdate(t, "v9.9.9", "v1.0.0", []byte("the new binary"))
	out, code := captureStdout(t, func() int { return runUpdate(nil) })
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "updated") || !strings.Contains(out, "v9.9.9") {
		t.Fatalf("output %q should report the update", out)
	}
	got, err := os.ReadFile(exe)
	if err != nil || string(got) != "the new binary" {
		t.Fatalf("binary = %q, %v", got, err)
	}
}

// Pinning a tag reinstalls it even when it is the version already running,
// which is how someone recovers from a half-broken install.
func TestUpdateVersionFlagReinstallsTheSameTag(t *testing.T) {
	exe := stubUpdate(t, "v9.9.9", "v9.9.9", []byte("reinstalled"))
	out, code := captureStdout(t, func() int { return runUpdate([]string{"--version", "v9.9.9"}) })
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if got, _ := os.ReadFile(exe); string(got) != "reinstalled" {
		t.Fatalf("binary = %q, want the pinned tag installed", got)
	}
}

func TestUpdateUnknownTagFails(t *testing.T) {
	exe := stubUpdate(t, "v9.9.9", "v1.0.0", []byte("new"))
	_, code := captureStdout(t, func() int { return runUpdate([]string{"--version", "v0.0.1"}) })
	if code == 0 {
		t.Fatal("want a non-zero exit for a tag that does not exist")
	}
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Fatalf("binary replaced on failure: %q", got)
	}
}

func TestUpdateRejectsPositionalArguments(t *testing.T) {
	stubUpdate(t, "v9.9.9", "v1.0.0", []byte("new"))
	if _, code := captureStdout(t, func() int { return runUpdate([]string{"now"}) }); code == 0 {
		t.Fatal("want a non-zero exit for a stray argument")
	}
}

func TestUpdateExplainsAPermissionProblem(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	exe := stubUpdate(t, "v9.9.9", "v1.0.0", []byte("new"))
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	stderr := captureStderr(t, func() {
		if _, code := captureStdout(t, func() int { return runUpdate(nil) }); code == 0 {
			t.Fatal("want a non-zero exit when the binary cannot be replaced")
		}
	})
	for _, want := range []string{"sudo lazyswap update", "get.lazyswap.app"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr %q should suggest %q", stderr, want)
		}
	}
}
