package core

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirSignature computes a deterministic content signature of a directory tree:
// a hash over every file's slash-relative path and contents. Any change —
// committed or not — alters the signature, so comparing it against the
// manifest's recorded value detects local divergence from the synced snapshot
// without needing the upstream's git objects locally.
//
// Noise that does not represent user intent is excluded: .git directories and
// .DS_Store files are skipped.
func DirSignature(root string) (string, error) {
	h := sha256.New()
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() || info.Name() == ".DS_Store" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		io.WriteString(h, rel)
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// dirHasFiles reports whether the directory contains at least one entry.
func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".DS_Store") {
			return true
		}
	}
	return false
}
