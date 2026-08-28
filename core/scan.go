package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// canonicalize resolves symlinks in a path (macOS /tmp → /private/tmp and
// similar); comparisons between resolved link targets and configured paths
// must happen in resolved space or they silently mismatch.
func canonicalize(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// Scan builds the full hub + targets picture: hub skills (top-level or nested
// one level), per-target entry classification (chained links are resolved
// through any number of hops), per-skill dirty files, and upstream metadata.
func Scan(hubPath string, targetPaths []string) (*State, error) {
	hub := canonicalize(mustAbs(hubPath))
	m, err := LoadManifest(hub)
	if err != nil {
		return nil, err
	}

	st := &State{Hub: hub, RepoOK: IsRepo(hub)}
	if st.RepoOK {
		lines, err := StatusPorcelain(hub)
		if err != nil {
			st.RepoOK = false
		} else {
			st.DirtyAll = porcelainPaths(lines)
		}
	}

	for _, sk := range scanHubSkills(hub) {
		sk.Description = readDescription(filepath.Join(hub, filepath.FromSlash(sk.Rel)))
		if meta, ok := m.Get(sk.Name); ok {
			sk.Upstream = Upstream{Source: meta.Source, Ref: meta.Ref, Synced: meta.Synced}
		}
		sk.Enabled = map[string]bool{}
		for _, tp := range targetPaths {
			sk.Enabled[tp] = false
		}
		for _, p := range st.DirtyAll {
			if p == sk.Rel || strings.HasPrefix(p, sk.Rel+"/") {
				sk.Dirty = append(sk.Dirty, p)
			}
		}
		st.Skills = append(st.Skills, sk)
	}
	sort.Slice(st.Skills, func(i, j int) bool { return st.Skills[i].Name < st.Skills[j].Name })

	byAbs := map[string]*Skill{} // skill dir abs path -> skill
	skillNames := map[string]bool{}
	for i := range st.Skills {
		byAbs[filepath.Join(hub, filepath.FromSlash(st.Skills[i].Rel))] = &st.Skills[i]
		skillNames[st.Skills[i].Name] = true
	}

	for i := range targetPaths {
		targetPaths[i] = canonicalize(targetPaths[i])
		tgt, err := scanTarget(hub, targetPaths[i], byAbs, skillNames)
		if err != nil {
			return nil, err
		}
		st.Targets = append(st.Targets, tgt)
	}
	return st, nil
}

func mustAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// scanHubSkills finds skills: top-level dirs with SKILL.md, or nested one level
// deeper (e.g. "ericadskill/draft-interview-intro-answers").
func scanHubSkills(hub string) []Skill {
	var out []Skill
	top, err := os.ReadDir(hub)
	if err != nil {
		return nil
	}
	hasSkillMD := func(dir string) bool {
		fi, err := os.Stat(filepath.Join(dir, "SKILL.md"))
		return err == nil && !fi.IsDir()
	}
	for _, e := range top {
		if !e.IsDir() {
			continue
		}
		abs := filepath.Join(hub, e.Name())
		switch {
		case hasSkillMD(abs):
			out = append(out, Skill{Name: e.Name(), Rel: e.Name()})
		default:
			nested, err := os.ReadDir(abs)
			if err != nil {
				continue
			}
			for _, n := range nested {
				if n.IsDir() && hasSkillMD(filepath.Join(abs, n.Name())) {
					out = append(out, Skill{Name: n.Name(), Rel: e.Name() + "/" + n.Name()})
				}
			}
		}
	}
	return out
}

// scanTarget classifies every entry of one target directory.
func scanTarget(hub string, targetPath string, byAbs map[string]*Skill, skillNames map[string]bool) (Target, error) {
	tgt := Target{Path: targetPath}
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			tgt.Missing = true
			return tgt, nil
		}
		return tgt, err
	}
	for _, e := range entries {
		linkPath := filepath.Join(targetPath, e.Name())
		fi, err := os.Lstat(linkPath)
		if err != nil {
			continue
		}
		entry := TargetEntry{Name: e.Name(), LinkPath: linkPath}
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			resolved, dangling := resolveLink(linkPath)
			if dangling {
				entry.Kind = KindBroken
				break
			}
			entry.Resolved = resolved
			if sk, ok := byAbs[resolved]; ok {
				entry.Kind = KindHubLink
				entry.HubSkill = sk.Rel
				sk.Enabled[targetPath] = true
			} else if insideDir(resolved, hub) {
				// resolves into the hub but not at a skill directory — treat as foreign
				entry.Kind = KindForeignLink
			} else {
				entry.Kind = KindForeignLink
			}
		case fi.IsDir():
			entry.Kind = KindForeignDir
			entry.LinkPath = ""
			entry.Conflicts = skillNames[e.Name()] // collides with a hub skill by name
		default:
			continue // regular files are not this tool's concern
		}
		tgt.Entries = append(tgt.Entries, entry)
	}
	sort.Slice(tgt.Entries, func(i, j int) bool { return tgt.Entries[i].Name < tgt.Entries[j].Name })
	return tgt, nil
}

// insideDir reports whether path is dir itself or anywhere beneath it.
func insideDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}
