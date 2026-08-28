package core

import (
	"os"
	"path/filepath"
)

// Enable creates the per-skill direct link target/<leaf> → hub/<rel>.
// Idempotent when the link already resolves to the same skill (chained links
// count: resolution goes through any number of hops); refuses to overwrite
// anything else (ErrConflict).
func Enable(hub, targetPath, rel string) error {
	leaf := filepath.Base(rel)
	skillDir := filepath.Join(hub, filepath.FromSlash(rel))
	linkPath := filepath.Join(targetPath, leaf)

	if fi, err := os.Lstat(linkPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			resolved, dangling := resolveLink(linkPath)
			if !dangling && resolved == skillDir {
				return nil // already enabled
			}
			return ErrConflict
		}
		return ErrConflict // real dir/file occupies the name
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(skillDir, linkPath)
}

// Disable removes the link for one skill in one target. Only links resolving
// into the hub are touched (chained links included — the single link segment
// in this target is removed, leaving any intermediate hops alone); real
// directories are never removed.
func Disable(hub, targetPath, leaf string) error {
	linkPath := filepath.Join(targetPath, leaf)
	fi, err := os.Lstat(linkPath)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return ErrNotLink
	}
	resolved, dangling := resolveLink(linkPath)
	if !dangling && !insideDir(resolved, hub) {
		return ErrOutsideHub
	}
	return os.Remove(linkPath)
}

// RemoveLink deletes any symlink entry (foreign or broken). Real directories
// are always refused. The TUI confirms with the user before calling this.
func RemoveLink(targetPath, leaf string) error {
	linkPath := filepath.Join(targetPath, leaf)
	fi, err := os.Lstat(linkPath)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return ErrNotLink
	}
	return os.Remove(linkPath)
}
