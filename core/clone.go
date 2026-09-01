package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CloneIntoHub clones a git repository, imports its supported skills as plain
// directories under hub, and records their upstream provenance. A supported
// repository contains SKILL.md at its root or in one or more direct children.
//
// The clone's .git directory is stripped before import so the hub repository
// versions the actual skill files instead of an embedded gitlink.
func CloneIntoHub(hub, source string) ([]string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmtErr("git URL is empty")
	}
	repoName, err := cloneRepoName(source)
	if err != nil {
		return nil, err
	}

	hub = canonicalize(mustAbs(hub))
	dest := filepath.Join(hub, repoName)
	if _, err := os.Lstat(dest); err == nil {
		return nil, fmtErr("%w: %s", ErrConflict, repoName)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	existingSkills := make(map[string]string)
	for _, sk := range scanHubSkills(hub) {
		existingSkills[sk.Name] = sk.Rel
	}

	staging, err := os.MkdirTemp(hub, ".skillsend-clone-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	cloneDir := filepath.Join(staging, repoName)
	if _, err := runGit("", "clone", "--depth", "1", source, cloneDir); err != nil {
		return nil, err
	}
	head, err := HeadSha(cloneDir)
	if err != nil {
		return nil, err
	}

	discovered := scanHubSkills(staging)
	if len(discovered) == 0 {
		return nil, fmtErr("repository contains no supported skills (expected SKILL.md at the root or in a direct child)")
	}
	for _, sk := range discovered {
		if existing, ok := existingSkills[sk.Name]; ok {
			return nil, fmtErr("%w: skill %q already exists at %s", ErrConflict, sk.Name, existing)
		}
	}

	manifest, err := LoadManifest(hub)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, sk := range discovered {
		skillDir := filepath.Join(staging, filepath.FromSlash(sk.Rel))
		sig, err := DirSignature(skillDir)
		if err != nil {
			return nil, err
		}
		upstreamPath := strings.TrimPrefix(filepath.ToSlash(sk.Rel), repoName)
		upstreamPath = strings.TrimPrefix(upstreamPath, "/")
		manifest.Set(sk.Name, SkillMeta{
			Source: source,
			Path:   upstreamPath,
			Synced: head,
			Tree:   sig,
		})
		names = append(names, sk.Name)
	}

	if err := os.RemoveAll(filepath.Join(cloneDir, ".git")); err != nil {
		return nil, err
	}
	if err := os.Rename(cloneDir, dest); err != nil {
		return nil, err
	}
	if err := manifest.Save(hub); err != nil {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	sort.Strings(names)
	return names, nil
}

func cloneRepoName(source string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(source), "/")
	if trimmed == "" {
		return "", fmtErr("invalid git URL %q", source)
	}
	if i := strings.LastIndexAny(trimmed, "/:"); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	name := strings.TrimSuffix(trimmed, ".git")
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmtErr("cannot determine repository name from %q", source)
	}
	return name, nil
}
