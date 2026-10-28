package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotWritable reports that the running binary's directory cannot be written
// by this user, so the update needs different privileges.
var ErrNotWritable = errors.New("no permission to replace the binary")

// ExePath returns the path of the running binary with symlinks resolved, which
// is what gets replaced.
func ExePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	// A package manager or a dotfiles setup may expose lazyswap through a
	// symlink; replacing the link itself would leave the real binary stale.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// Install downloads rel, verifies it and moves it over exePath.
func (u Updater) Install(ctx context.Context, rel Release, exePath string) error {
	dir := filepath.Dir(exePath)
	// Check the permission before spending a download on it, and report it as
	// its own error so the caller can explain what to do.
	if err := checkWritable(dir); err != nil {
		return err
	}

	tarball, err := u.downloadTemp(ctx, dir, rel.Tarball)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tarball) }()

	want, err := u.fetchChecksum(ctx, rel.Checksum)
	if err != nil {
		return err
	}
	got, err := sha256File(tarball)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: published %s, downloaded %s — not installing", rel.Tag, want, got)
	}

	staged, err := extractBinary(tarball, dir)
	if err != nil {
		return err
	}
	// Rename within one directory is atomic: either the old binary or the new
	// one is there, never a partial file. The running process keeps its own
	// open inode, so replacing it underneath is safe.
	if err := os.Rename(staged, exePath); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("replace %s: %w", exePath, err)
	}
	return nil
}

// checkWritable probes the directory rather than the binary: on Linux opening
// a running executable for writing fails with ETXTBSY even when the
// permissions are fine, and the update replaces the file by renaming into this
// directory anyway.
func checkWritable(dir string) error {
	probe, err := os.CreateTemp(dir, ".lazyswap-probe-*")
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("%w: %s is not writable by this user", ErrNotWritable, dir)
		}
		return err
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

func (u Updater) downloadTemp(ctx context.Context, dir, rawURL string) (string, error) {
	body, err := u.get(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer func() { _ = body.Close() }()

	// 0600 while it is unverified: nothing else should be able to read, and
	// certainly not execute, a binary that has not been checked yet.
	f, err := os.CreateTemp(dir, ".lazyswap-download-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := io.Copy(f, io.LimitReader(body, maxBinaryBytes)); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", fmt.Errorf("download: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// fetchChecksum reads the published sha256 file, whose format is the output of
// sha256sum: "<hex>  <filename>".
func (u Updater) fetchChecksum(ctx context.Context, rawURL string) (string, error) {
	body, err := u.get(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("download checksum: %w", err)
	}
	defer func() { _ = body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(body, maxMetaBytes))
	if err != nil {
		return "", fmt.Errorf("read checksum: %w", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return "", errors.New("the published checksum file is empty")
	}
	sum := strings.ToLower(fields[0])
	if decoded, err := hex.DecodeString(sum); err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("the published checksum is not a sha256: %q", fields[0])
	}
	return sum, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractBinary writes the archive's "lazyswap" entry to a new executable file
// in dir and returns its path. Every other entry is refused rather than
// skipped: a release tarball holds exactly one file, so anything else means
// the archive is not what we think it is.
func extractBinary(tarball, dir string) (string, error) {
	f, err := os.Open(tarball)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if errors.Is(err, io.EOF) {
		return "", fmt.Errorf("the archive has no %s binary", binaryName)
	}
	if err != nil {
		return "", fmt.Errorf("read archive: %w", err)
	}
	if hdr.Name != binaryName {
		return "", fmt.Errorf("unexpected entry %q in the archive — refusing it", hdr.Name)
	}
	if hdr.Typeflag != tar.TypeReg {
		return "", fmt.Errorf("%s in the archive is not a regular file", binaryName)
	}
	staged, err := writeExecutable(tr, dir)
	if err != nil {
		return "", err
	}
	// A release tarball holds exactly one file. Anything trailing means this
	// is not the archive we think it is.
	if _, err := tr.Next(); !errors.Is(err, io.EOF) {
		_ = os.Remove(staged)
		return "", errors.New("the archive holds more than the binary — refusing it")
	}
	return staged, nil
}

func writeExecutable(r io.Reader, dir string) (string, error) {
	out, err := os.CreateTemp(dir, ".lazyswap-staged-*")
	if err != nil {
		return "", err
	}
	name := out.Name()
	fail := func(err error) (string, error) {
		_ = out.Close()
		_ = os.Remove(name)
		return "", err
	}
	if _, err := io.Copy(out, io.LimitReader(r, maxBinaryBytes)); err != nil {
		return fail(fmt.Errorf("extract: %w", err))
	}
	if err := out.Chmod(0o755); err != nil {
		return fail(err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
