// Package core implements Skillsend's state model and mutating operations.
// It is the single test seam: the TUI is a thin shell over it.
package core

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// EntryKind classifies an entry found in a target directory.
type EntryKind int

const (
	// KindHubLink is a symlink that resolves (through any number of hops) into the hub.
	KindHubLink EntryKind = iota
	// KindForeignLink is a symlink resolving somewhere else.
	KindForeignLink
	// KindBroken is a dangling symlink.
	KindBroken
	// KindForeignDir is a real directory (never touched except via explicit adopt).
	KindForeignDir
)

func (k EntryKind) String() string {
	switch k {
	case KindHubLink:
		return "hub-link"
	case KindForeignLink:
		return "foreign-link"
	case KindBroken:
		return "broken"
	case KindForeignDir:
		return "foreign-dir"
	}
	return "unknown"
}

// TargetEntry is one entry inside a target directory.
type TargetEntry struct {
	Name      string // entry name within the target dir
	Kind      EntryKind
	LinkPath  string // absolute path of the symlink; empty for foreign-dir
	Resolved  string // final resolved absolute path; "" when broken
	HubSkill  string // rel path of the matching hub skill (hub-link only)
	Conflicts bool   // foreign-dir whose name collides with a hub skill
}

// Target is a link destination directory (~/.agents/skills, ~/.claude/skills).
type Target struct {
	Path    string
	Entries []TargetEntry
	Missing bool // directory does not exist (never auto-created)
}

// Upstream records where a third-party skill came from and the last synced snapshot.
type Upstream struct {
	Source string // git URL
	Ref    string // branch/tag, empty = remote default
	Synced string // upstream commit sha the local content last matched
}

// Empty reports whether there is no upstream recorded for this skill.
func (u Upstream) Empty() bool { return u.Source == "" }

// Skill is one skill managed in the hub.
type Skill struct {
	Name        string // leaf directory name; also the symlink name in targets
	Rel         string // hub-root-relative path, e.g. "ericadskill/draft-interview-intro-answers"
	Description string // from SKILL.md frontmatter
	Upstream    Upstream
	Dirty       []string        // uncommitted hub files under this skill (rel to hub root)
	Enabled     map[string]bool // target path -> a link for this skill exists there
}

// State is the full picture of hub + targets, rebuilt by Scan.
type State struct {
	Hub      string
	Skills   []Skill
	Targets  []Target
	RepoOK   bool
	DirtyAll []string // all uncommitted hub files (rel to hub root)
}

// SkillByName returns the skill with the given leaf name.
func (s *State) SkillByName(name string) (Skill, bool) {
	for _, sk := range s.Skills {
		if sk.Name == name {
			return sk, true
		}
	}
	return Skill{}, false
}

// hubSkillMatches reports whether resolved points inside the hub at the given skill rel path.
func hubSkillMatches(hub, rel, resolved string) bool {
	want := filepath.Join(hub, filepath.FromSlash(rel))
	return resolved == want
}

// resolveLink follows a symlink to its final target. Returns the resolved path and
// whether it is dangling.
func resolveLink(linkPath string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return "", true
	}
	return resolved, false
}

// sortedNames returns dir entry names in deterministic order.
func sortedNames[T any](m map[string]T) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// splitFrontmatter splits a SKILL.md into frontmatter lines and body.
func splitFrontmatter(data string) (map[string]string, string) {
	lines := strings.Split(data, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, data
	}
	meta := map[string]string{}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
		key, val, ok := strings.Cut(lines[i], ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key != "" {
			meta[key] = val
		}
	}
	if end == -1 {
		return nil, data
	}
	return meta, strings.Join(lines[end+1:], "\n")
}

func fmtErr(format string, args ...any) error { return fmt.Errorf(format, args...) }
