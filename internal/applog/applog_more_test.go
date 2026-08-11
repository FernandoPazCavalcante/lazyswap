package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteLevelsToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	SetPath(path)
	t.Cleanup(func() { SetPath("/dev/null") })

	Info("hello info")
	Infof("hello %s", "infof")
	Warn("hello warn")
	Warnf("hello %s", "warnf")
	Error("hello error", os.ErrNotExist)
	Errorf("hello %s", "errorf")
	Trace("hello trace")
	Tracef("hello %s", "tracef")

	if Path() != path {
		t.Fatalf("Path() = %q", Path())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	out := string(b)
	for _, marker := range []string{
		"INFO", "hello info", "hello infof",
		"WARN", "hello warnf",
		"ERROR", "hello error", "file does not exist", "hello errorf",
		"TRACE", "hello tracef",
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("log missing %q:\n%s", marker, out)
		}
	}
}

func TestDevNullNeverFails(t *testing.T) {
	SetPath("/dev/null")
	Info("dropped")
	Warn("dropped") // must not panic or create files
}
