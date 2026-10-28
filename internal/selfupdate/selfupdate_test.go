package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarGz builds a release-shaped archive holding one entry.
func tarGz(t *testing.T, name string, body []byte, typeflag byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: typeflag}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if typeflag == tar.TypeReg {
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// tarGzTwo builds an archive whose first entry is the real binary and whose
// second is something else.
func tarGzTwo(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{binaryName, "extra"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// server stands in for the GitHub API and the release download. stableTag is
// served from /releases/latest; when it is empty that endpoint 404s, which is
// what GitHub does for a project with only prereleases.
type server struct {
	t         *testing.T
	stableTag string
	listTag   string
	archive   []byte
	checksum  string // served verbatim; empty means "serve the real one"
	omit      string // asset name to leave out of the release JSON
}

func (s *server) start() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(s.handle))
	s.t.Cleanup(srv.Close)
	return srv
}

func (s *server) releaseJSON(base, tag string) string {
	var assets []string
	for _, name := range []string{"lazyswap-linux-x64.tar.gz", "lazyswap-linux-x64.tar.gz.sha256"} {
		if name == s.omit {
			continue
		}
		assets = append(assets, fmt.Sprintf(`{"name":%q,"browser_download_url":"%s/dl/%s"}`, name, base, name))
	}
	return fmt.Sprintf(`{"tag_name":%q,"assets":[%s]}`, tag, strings.Join(assets, ","))
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	base := "http://" + r.Host
	switch {
	case r.URL.Path == "/repos/o/p/releases/latest":
		if s.stableTag == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(s.releaseJSON(base, s.stableTag)))
	case r.URL.Path == "/repos/o/p/releases":
		if s.listTag == "" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte("[" + s.releaseJSON(base, s.listTag) + "]"))
	case strings.HasPrefix(r.URL.Path, "/repos/o/p/releases/tags/"):
		tag := strings.TrimPrefix(r.URL.Path, "/repos/o/p/releases/tags/")
		if tag != s.stableTag && tag != s.listTag {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(s.releaseJSON(base, tag)))
	case r.URL.Path == "/dl/lazyswap-linux-x64.tar.gz":
		_, _ = w.Write(s.archive)
	case r.URL.Path == "/dl/lazyswap-linux-x64.tar.gz.sha256":
		if s.checksum != "" {
			_, _ = w.Write([]byte(s.checksum))
			return
		}
		_, _ = fmt.Fprintf(w, "%s  lazyswap-linux-x64.tar.gz\n", sum(s.archive))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newUpdater(base string) Updater {
	return Updater{APIBase: base, Repo: "o/p", GOOS: "linux", GOARCH: "amd64"}
}

func TestTargetMapsSupportedPlatformsOnly(t *testing.T) {
	ok := map[[2]string]string{
		{"linux", "amd64"}:  "linux-x64",
		{"linux", "arm64"}:  "linux-arm64",
		{"darwin", "amd64"}: "darwin-x64",
		{"darwin", "arm64"}: "darwin-arm64",
	}
	for in, want := range ok {
		got, err := Target(in[0], in[1])
		if err != nil || got != want {
			t.Errorf("Target(%q,%q) = %q, %v; want %q", in[0], in[1], got, err, want)
		}
	}
	for _, in := range [][2]string{{"windows", "amd64"}, {"linux", "386"}, {"plan9", "arm64"}} {
		if _, err := Target(in[0], in[1]); err == nil {
			t.Errorf("Target(%q,%q) should be unsupported", in[0], in[1])
		}
	}
}

func TestLatestPrefersStableAndFallsBackToPrerelease(t *testing.T) {
	archive := tarGz(t, binaryName, []byte("binary"), tar.TypeReg)

	s := &server{t: t, stableTag: "v2.0.0", listTag: "v2.1.0-beta.1", archive: archive}
	rel, err := newUpdater(s.start().URL).Latest(context.Background())
	if err != nil || rel.Tag != "v2.0.0" {
		t.Fatalf("with a stable release: got %q, %v; want v2.0.0", rel.Tag, err)
	}

	// No stable release yet — exactly where this project is today.
	s2 := &server{t: t, listTag: "v1.0.0-beta.2", archive: archive}
	rel, err = newUpdater(s2.start().URL).Latest(context.Background())
	if err != nil || rel.Tag != "v1.0.0-beta.2" {
		t.Fatalf("prerelease fallback: got %q, %v; want v1.0.0-beta.2", rel.Tag, err)
	}
	if rel.Tarball == "" || rel.Checksum == "" {
		t.Fatalf("release is missing a URL: %+v", rel)
	}
}

func TestLatestReportsNoReleases(t *testing.T) {
	s := &server{t: t, archive: nil}
	if _, err := newUpdater(s.start().URL).Latest(context.Background()); err == nil {
		t.Fatal("want an error when the project has no releases")
	}
}

func TestResolveTag(t *testing.T) {
	s := &server{t: t, stableTag: "v2.0.0", archive: tarGz(t, binaryName, []byte("x"), tar.TypeReg)}
	u := newUpdater(s.start().URL)
	rel, err := u.Resolve(context.Background(), "v2.0.0")
	if err != nil || rel.Tag != "v2.0.0" {
		t.Fatalf("Resolve: got %q, %v", rel.Tag, err)
	}
	if _, err := u.Resolve(context.Background(), "v9.9.9"); err == nil {
		t.Fatal("want an error for a tag that does not exist")
	}
	if _, err := u.Resolve(context.Background(), ""); err == nil {
		t.Fatal("want an error for an empty tag")
	}
}

func TestPickRefusesAReleaseWithoutThisPlatformOrAChecksum(t *testing.T) {
	archive := tarGz(t, binaryName, []byte("x"), tar.TypeReg)

	s := &server{t: t, stableTag: "v2.0.0", archive: archive, omit: "lazyswap-linux-x64.tar.gz"}
	_, err := newUpdater(s.start().URL).Latest(context.Background())
	if !errors.Is(err, ErrNoAsset) {
		t.Fatalf("missing tarball: err = %v, want ErrNoAsset", err)
	}

	// An unverifiable binary must not install, even though the tarball is there.
	s2 := &server{t: t, stableTag: "v2.0.0", archive: archive, omit: "lazyswap-linux-x64.tar.gz.sha256"}
	_, err = newUpdater(s2.start().URL).Latest(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no checksum") {
		t.Fatalf("missing checksum: err = %v, want a refusal", err)
	}
}

// stageExe writes a stand-in for the running binary and returns its path.
func stageExe(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), binaryName)
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstallReplacesTheBinary(t *testing.T) {
	payload := []byte("the new binary")
	s := &server{t: t, listTag: "v1.0.0-beta.3", archive: tarGz(t, binaryName, payload, tar.TypeReg)}
	u := newUpdater(s.start().URL)
	rel, err := u.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	exe := stageExe(t)
	if err := u.Install(context.Background(), rel, exe); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("binary = %q, %v; want %q", got, err, payload)
	}
	info, err := os.Stat(exe)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, %v; want 0755", info.Mode().Perm(), err)
	}
	// No download or staging leftovers beside it.
	entries, err := os.ReadDir(filepath.Dir(exe))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("leftover files next to the binary: %v", entries)
	}
}

func TestInstallRefusesABadArchive(t *testing.T) {
	good := []byte("payload")
	cases := []struct {
		name    string
		archive []byte
		sum     string
		wantIn  string
	}{
		{
			name:    "checksum mismatch",
			archive: tarGz(t, binaryName, good, tar.TypeReg),
			sum:     strings.Repeat("ab", 32) + "  lazyswap-linux-x64.tar.gz\n",
			wantIn:  "checksum mismatch",
		},
		{
			name:    "checksum is not a sha256",
			archive: tarGz(t, binaryName, good, tar.TypeReg),
			sum:     "not-a-hash  lazyswap-linux-x64.tar.gz\n",
			wantIn:  "not a sha256",
		},
		{
			name:    "empty checksum file",
			archive: tarGz(t, binaryName, good, tar.TypeReg),
			sum:     "   \n",
			wantIn:  "empty",
		},
		{
			name:    "entry escaping the directory",
			archive: tarGz(t, "../../etc/profile", good, tar.TypeReg),
			wantIn:  "unexpected entry",
		},
		{
			name:    "entry with another name",
			archive: tarGz(t, "lazyswap-helper", good, tar.TypeReg),
			wantIn:  "unexpected entry",
		},
		{
			name:    "entry is a directory",
			archive: tarGz(t, binaryName, nil, tar.TypeDir),
			wantIn:  "not a regular file",
		},
		{
			name:    "not an archive at all",
			archive: []byte("this is not gzip"),
			wantIn:  "open archive",
		},
		{
			name:    "more than the binary inside",
			archive: tarGzTwo(t, good),
			wantIn:  "more than the binary",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &server{t: t, listTag: "v1.0.0-beta.3", archive: tc.archive, checksum: tc.sum}
			u := newUpdater(s.start().URL)
			rel, err := u.Latest(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			exe := stageExe(t)
			err = u.Install(context.Background(), rel, exe)
			if err == nil {
				t.Fatal("Install should have refused this archive")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.wantIn)
			}
			// The old binary has to survive every refusal.
			got, readErr := os.ReadFile(exe)
			if readErr != nil || string(got) != "old binary" {
				t.Fatalf("the original binary was damaged: %q, %v", got, readErr)
			}
			entries, _ := os.ReadDir(filepath.Dir(exe))
			if len(entries) != 1 {
				t.Fatalf("temporary files were left behind: %v", entries)
			}
		})
	}
}

func TestInstallReportsAnUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	s := &server{t: t, listTag: "v1", archive: tarGz(t, binaryName, []byte("x"), tar.TypeReg)}
	u := newUpdater(s.start().URL)
	rel, err := u.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, binaryName)
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err = u.Install(context.Background(), rel, exe)
	if !errors.Is(err, ErrNotWritable) {
		t.Fatalf("err = %v, want ErrNotWritable", err)
	}
}

func TestProductionBuildRefusesPlainHTTP(t *testing.T) {
	// APIBase empty is the released configuration: no http, ever.
	u := Updater{Repo: "o/p", GOOS: "linux", GOARCH: "amd64"}
	_, err := u.get(context.Background(), "http://example.invalid/x")
	if err == nil || !strings.Contains(err.Error(), "non-https") {
		t.Fatalf("err = %v, want a refusal of plain http", err)
	}
}

func TestHTTPSOnlyRedirectGuard(t *testing.T) {
	if err := httpsOnly(httptest.NewRequest("GET", "https://ok.example/x", nil), nil); err != nil {
		t.Fatalf("https redirect rejected: %v", err)
	}
	if err := httpsOnly(httptest.NewRequest("GET", "http://bad.example/x", nil), nil); err == nil {
		t.Fatal("want http redirects refused")
	}
	long := make([]*http.Request, 10)
	if err := httpsOnly(httptest.NewRequest("GET", "https://ok.example/x", nil), long); err == nil {
		t.Fatal("want a redirect-chain limit")
	}
}
