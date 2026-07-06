package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// destState classifies what currently sits at a component's destination path,
// which determines the choices offered to the user.
type destState int

const (
	destFree       destState = iota // nothing there — clean install
	destLinkedToUs                  // symlink already pointing at our source (installed)
	destLinkedElse                  // symlink pointing somewhere else
	destOccupied                    // a real file or directory
)

// resolution is the action chosen for a single component at install time.
type resolution int

const (
	resInstall   resolution = iota // create the symlink (dest is free)
	resSkip                        // leave the destination untouched
	resOverwrite                   // delete whatever is there, then symlink
	resBackup                      // rename existing to .bak-N, then symlink
	resRemove                      // delete our existing symlink (uninstall)
)

// destPath returns the absolute destination for a component under a target
// .claude/ root.
func destPath(targetClaude string, c Component) string {
	return filepath.Join(targetClaude, c.RelPath)
}

// classifyDest inspects the destination and reports its state plus, when it is
// a symlink, the (resolved) path it points at.
func classifyDest(targetClaude string, c Component) (destState, string) {
	dest := destPath(targetClaude, c)
	info, err := os.Lstat(dest)
	if err != nil {
		return destFree, "" // ErrNotExist (or unreadable parent) — treat as free
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return destOccupied, "" // real file or directory
	}
	// It's a symlink. Compare where it resolves to against our source.
	target, err := filepath.EvalSymlinks(dest)
	if err != nil {
		return destLinkedElse, "" // dangling symlink — not ours to trust
	}
	source, err := filepath.EvalSymlinks(c.Source)
	if err != nil {
		source = c.Source
	}
	if target == source {
		return destLinkedToUs, target
	}
	return destLinkedElse, target
}

// choicesFor returns the resolutions a user may pick for a given dest state,
// in menu order. The first entry is the sensible default.
func choicesFor(s destState) []resolution {
	switch s {
	case destFree:
		return []resolution{resInstall}
	case destLinkedToUs:
		return []resolution{resSkip, resRemove}
	default: // destLinkedElse, destOccupied
		return []resolution{resSkip, resOverwrite, resBackup}
	}
}

// apply performs the chosen resolution for one component and returns a short
// human-readable result line.
func apply(targetClaude string, c Component, r resolution) (string, error) {
	dest := destPath(targetClaude, c)
	switch r {
	case resSkip:
		return "skipped", nil
	case resRemove:
		if err := os.Remove(dest); err != nil {
			return "", err
		}
		return "removed (unlinked)", nil
	case resOverwrite:
		if err := os.RemoveAll(dest); err != nil {
			return "", err
		}
		return linkResult(c.Source, dest, "overwrote → linked")
	case resBackup:
		bak, err := backupPath(dest)
		if err != nil {
			return "", err
		}
		if err := os.Rename(dest, bak); err != nil {
			return "", err
		}
		return linkResult(c.Source, dest, fmt.Sprintf("backed up %s → linked", filepath.Base(bak)))
	case resInstall:
		return linkResult(c.Source, dest, "linked")
	}
	return "", fmt.Errorf("unknown resolution")
}

// linkResult ensures the destination's parent exists and creates the symlink.
func linkResult(source, dest, msg string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.Symlink(source, dest); err != nil {
		return "", err
	}
	return msg, nil
}

// backupPath finds the first free ".bak-N" name for dest.
func backupPath(dest string) (string, error) {
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s.bak-%d", dest, i)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free backup name for %s", dest)
}
