package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/selfupdate"
)

// newUpdater and exePath are vars so tests can point the update at a local
// server and a throwaway file. Nothing reads them from the environment: see
// the selfupdate package comment on why the download origin is fixed.
var (
	newUpdater = func() selfupdate.Updater { return selfupdate.Updater{} }
	exePath    = selfupdate.ExePath
)

// runUpdate replaces the running binary with a published release.
func runUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "report what is available, install nothing")
	pin := fs.String("version", "", "install this exact release tag instead of the newest")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		return die("update takes no arguments (see --check and --version)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	u := newUpdater()
	rel, err := resolveRelease(ctx, u, *pin)
	if err != nil {
		return die("%v", err)
	}

	fmt.Printf("installed: %s\nlatest:    %s\n", version, rel.Tag)
	if *check {
		if rel.Tag == version {
			fmt.Println("up to date")
		}
		return 0
	}
	if rel.Tag == version && *pin == "" {
		fmt.Println("already up to date")
		return 0
	}
	if version == "dev" {
		fmt.Println("note: this binary reports version \"dev\", so it was built from source rather than installed from a release — it will be replaced by the released build")
	}

	path, err := exePath()
	if err != nil {
		return die("locate the running binary: %v", err)
	}
	fmt.Printf("downloading %s…\n", rel.Tag)
	if err := u.Install(ctx, rel, path); err != nil {
		if errors.Is(err, selfupdate.ErrNotWritable) {
			return die("%v\n\n  retry with elevated privileges, for example:\n    sudo lazyswap update\n\n  or reinstall with:\n    curl -sSL https://get.lazyswap.app | bash", err)
		}
		return die("%v", err)
	}
	fmt.Printf("updated %s to %s\n", path, rel.Tag)
	return 0
}

func resolveRelease(ctx context.Context, u selfupdate.Updater, pin string) (selfupdate.Release, error) {
	if pin != "" {
		return u.Resolve(ctx, pin)
	}
	return u.Latest(ctx)
}
