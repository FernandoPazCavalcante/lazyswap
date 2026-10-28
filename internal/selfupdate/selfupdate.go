// Package selfupdate resolves the newest published release and replaces the
// running binary with it.
//
// This is a download-and-execute path inside a program that holds private
// keys, so the rules it follows are deliberate:
//
//   - The download origin is not configurable at runtime. There is no
//     environment variable for it; only a test sets APIBase. A settable
//     origin would be a way to get an attacker's binary installed.
//   - Every redirect must stay on https.
//   - The tarball is checked against the SHA-256 published beside it, and
//     nothing is extracted until it matches. Note what that does and does not
//     buy: it catches a truncated or corrupted download and a tampered object
//     store, but the checksum comes from the same origin as the tarball, so it
//     is not protection against a compromised release account. Signed releases
//     with a pinned key would be; see SECURITY.md.
//   - Only a regular file entry named exactly "lazyswap" is read out of the
//     archive. Any other path, including one trying to escape the directory,
//     is refused.
//   - The new binary lands in the same directory as the old one and is moved
//     into place with rename(2), so an interrupted update leaves the previous
//     binary intact rather than a half-written one.
package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

const (
	defaultAPIBase = "https://api.github.com"
	defaultRepo    = "FernandoPazCavalcante/lazyswap"

	// binaryName is the only entry read out of a release tarball.
	binaryName = "lazyswap"

	// maxBinaryBytes bounds what gzip is allowed to expand to. The real
	// binary is around 20 MB; this is a decompression-bomb guard, not a
	// size limit anyone should hit.
	maxBinaryBytes = 200 << 20

	// maxMetaBytes bounds the API responses and the checksum file.
	maxMetaBytes = 1 << 20
)

// ErrNoAsset reports that the newest release has no build for this platform.
var ErrNoAsset = errors.New("no build for this platform in that release")

// Release is a published release with the two URLs this platform needs.
type Release struct {
	Tag      string
	Tarball  string
	Checksum string
}

// Updater resolves and installs releases.
type Updater struct {
	// APIBase is the GitHub API root. Only tests set it; see the package
	// comment on why there is no environment override.
	APIBase string
	Repo    string
	HTTP    *http.Client
	GOOS    string
	GOARCH  string
}

func (u Updater) apiBase() string {
	if u.APIBase != "" {
		return u.APIBase
	}
	return defaultAPIBase
}

func (u Updater) repo() string {
	if u.Repo != "" {
		return u.Repo
	}
	return defaultRepo
}

func (u Updater) goos() string {
	if u.GOOS != "" {
		return u.GOOS
	}
	return runtime.GOOS
}

func (u Updater) goarch() string {
	if u.GOARCH != "" {
		return u.GOARCH
	}
	return runtime.GOARCH
}

// httpsOnly refuses a redirect that leaves https. The release download
// redirects to an object store, so redirects have to be followed — just not
// downgraded.
func httpsOnly(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("too many redirects")
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect to %s (not https)", req.URL.Scheme)
	}
	return nil
}

func (u Updater) client() *http.Client {
	if u.HTTP != nil {
		return u.HTTP
	}
	return &http.Client{Timeout: 2 * time.Minute, CheckRedirect: httpsOnly}
}

// Target maps a Go platform onto the name used in the release assets.
func Target(goos, goarch string) (string, error) {
	var osPart string
	switch goos {
	case "linux", "darwin":
		osPart = goos
	default:
		return "", fmt.Errorf("unsupported OS %q (releases cover linux and darwin)", goos)
	}
	var archPart string
	switch goarch {
	case "amd64":
		archPart = "x64"
	case "arm64":
		archPart = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture %q (releases cover x64 and arm64)", goarch)
	}
	return osPart + "-" + archPart, nil
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	Tag    string    `json:"tag_name"`
	Assets []ghAsset `json:"assets"`
}

// Latest returns the newest published release. It mirrors what the install
// script does: prefer a stable release, and fall back to the most recent one
// including prereleases, because the project has not tagged a stable version
// yet.
func (u Updater) Latest(ctx context.Context) (Release, error) {
	rel, err := u.fetchRelease(ctx, u.apiBase()+"/repos/"+u.repo()+"/releases/latest")
	if err == nil {
		return u.pick(rel)
	}
	var status statusError
	if !errors.As(err, &status) || status.code != http.StatusNotFound {
		return Release{}, err
	}
	list, err := u.fetchReleaseList(ctx, u.apiBase()+"/repos/"+u.repo()+"/releases?per_page=1")
	if err != nil {
		return Release{}, err
	}
	if len(list) == 0 {
		return Release{}, errors.New("the project has published no releases")
	}
	return u.pick(list[0])
}

// Resolve returns one exact release tag.
func (u Updater) Resolve(ctx context.Context, tag string) (Release, error) {
	if tag == "" {
		return Release{}, errors.New("empty release tag")
	}
	rel, err := u.fetchRelease(ctx, u.apiBase()+"/repos/"+u.repo()+"/releases/tags/"+url.PathEscape(tag))
	if err != nil {
		var status statusError
		if errors.As(err, &status) && status.code == http.StatusNotFound {
			return Release{}, fmt.Errorf("no release tagged %s", tag)
		}
		return Release{}, err
	}
	return u.pick(rel)
}

// pick finds this platform's tarball and its checksum in a release.
func (u Updater) pick(rel ghRelease) (Release, error) {
	if rel.Tag == "" {
		return Release{}, errors.New("release has no tag")
	}
	target, err := Target(u.goos(), u.goarch())
	if err != nil {
		return Release{}, err
	}
	want := binaryName + "-" + target + ".tar.gz"
	out := Release{Tag: rel.Tag}
	for _, a := range rel.Assets {
		switch a.Name {
		case want:
			out.Tarball = a.URL
		case want + ".sha256":
			out.Checksum = a.URL
		}
	}
	if out.Tarball == "" {
		return Release{}, fmt.Errorf("%w: %s has no %s", ErrNoAsset, rel.Tag, want)
	}
	if out.Checksum == "" {
		// Refusing is the point: an unverifiable binary is worse than no
		// update, and every release the CI publishes has a checksum.
		return Release{}, fmt.Errorf("%s publishes no checksum for %s — refusing to install it", rel.Tag, want)
	}
	return out, nil
}

type statusError struct {
	code int
	url  string
}

func (e statusError) Error() string { return fmt.Sprintf("GET %s: HTTP %d", e.url, e.code) }

func (u Updater) get(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	// Only a test sets APIBase, and only a test may therefore reach a plain
	// http server. In a released binary every URL here must be https.
	if u.APIBase == "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme != "https" {
			return nil, fmt.Errorf("refusing a non-https URL: %s", rawURL)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lazyswap-selfupdate")
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, statusError{code: resp.StatusCode, url: rawURL}
	}
	return resp.Body, nil
}

func (u Updater) fetchRelease(ctx context.Context, rawURL string) (ghRelease, error) {
	body, err := u.get(ctx, rawURL)
	if err != nil {
		return ghRelease{}, err
	}
	defer func() { _ = body.Close() }()
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(body, maxMetaBytes)).Decode(&rel); err != nil {
		return ghRelease{}, fmt.Errorf("decode release: %w", err)
	}
	return rel, nil
}

func (u Updater) fetchReleaseList(ctx context.Context, rawURL string) ([]ghRelease, error) {
	body, err := u.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	var list []ghRelease
	if err := json.NewDecoder(io.LimitReader(body, maxMetaBytes)).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}
	return list, nil
}
