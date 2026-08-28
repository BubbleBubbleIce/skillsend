package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Adopt moves a foreign real-directory skill into the hub and replaces its
// original location with a direct link — an explicit, user-confirmed operation.
// When the source lived in (or was) a git repo with an origin remote, the
// upstream provenance is recorded in the manifest so the skill becomes updatable.
func Adopt(srcPath, hub string, m *Manifest) (SkillMeta, error) {
	hub = canonicalize(mustAbs(hub))
	// Canonicalize only the parent: resolving src itself would hide the
	// symlink-ness we must reject, and /tmp-style parent symlinks must still go.
	parent := canonicalize(filepath.Dir(mustAbs(srcPath)))
	src := filepath.Join(parent, filepath.Base(mustAbs(srcPath)))
	fi, err := os.Lstat(src)
	if err != nil {
		return SkillMeta{}, err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return SkillMeta{}, ErrNotLink
	}
	leaf := filepath.Base(src)
	dest := filepath.Join(hub, leaf)
	if _, err := os.Lstat(dest); err == nil {
		return SkillMeta{}, ErrConflict
	} else if !os.IsNotExist(err) {
		return SkillMeta{}, err
	}

	// Capture provenance before moving: origin repo + the skill's path inside it.
	_, meta, err := upstreamProvenance(src)
	if err != nil {
		return SkillMeta{}, err
	}

	if err := os.Rename(src, dest); err != nil {
		if !errors.As(err, new(*os.LinkError)) {
			return SkillMeta{}, err
		}
		// cross-device: copy then remove
		if err := copyTree(src, dest); err != nil {
			return SkillMeta{}, err
		}
		if err := os.RemoveAll(src); err != nil {
			return SkillMeta{}, err
		}
	}

	// Strip any nested .git: provenance is already captured, and a nested repo
	// would make the hub track a gitlink instead of versioning the content.
	if err := os.RemoveAll(filepath.Join(dest, ".git")); err != nil {
		return meta, err
	}

	if meta.Source != "" {
		sig, err := DirSignature(dest)
		if err != nil {
			return SkillMeta{}, err
		}
		meta.Tree = sig
		m.Set(leaf, meta)
		if err := m.Save(hub); err != nil {
			return meta, err
		}
	}

	if err := os.Symlink(dest, src); err != nil {
		// Roll back: restore the original directory (same volume first, copy
		// across devices as fallback) and undo the manifest entry.
		if rbErr := os.Rename(dest, src); rbErr != nil {
			if cpErr := copyTree(dest, src); cpErr != nil {
				return meta, fmtErr("symlink failed (%v) and rollback failed (%v)", err, cpErr)
			}
			os.RemoveAll(dest)
		}
		m.Remove(leaf)
		_ = m.Save(hub)
		return meta, err
	}
	return meta, nil
}

// upstreamProvenance inspects the git context around src to build a manifest
// entry: the skill's own repo (root layout) or a parent collection repo
// (subdirectory layout). Returns the repo root and the meta (Source empty when
// no git provenance exists).
func upstreamProvenance(src string) (string, SkillMeta, error) {
	var meta SkillMeta
	rootOut, err := exec.Command("git", "-C", src, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", meta, nil // not inside a git repo — a personal skill, no provenance
	}
	root := canonicalize(strings.TrimSpace(string(rootOut)))
	urlOut, err := exec.Command("git", "-C", root, "config", "--get", "remote.origin.url").Output()
	if err != nil || strings.TrimSpace(string(urlOut)) == "" {
		return root, meta, nil // repo but no origin — nothing to pull from later
	}
	meta.Source = strings.TrimSpace(string(urlOut))
	rel, err := filepath.Rel(root, src)
	if err != nil {
		return root, meta, nil
	}
	if rel != "." {
		meta.Path = filepath.ToSlash(rel)
	}
	sha, err := HeadSha(root)
	if err == nil {
		meta.Synced = sha
	}
	return root, meta, nil
}
