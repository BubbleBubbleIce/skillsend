package core

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Manifest is the hub-root skillsend.toml: per-skill upstream metadata.
// Third-party skill content itself stays a plain directory versioned by the hub
// repo; this file is the only place recording where it came from.
type Manifest struct {
	Skills map[string]SkillMeta `toml:"skills"`
}

// SkillMeta is the upstream record for one skill, keyed by leaf name.
// Path is the skill's subdirectory inside the upstream repo ("" = repo root) —
// upstream repos are often collections. Tree is the content signature (see
// DirSignature) of the skill directory as it was when last synced with the
// upstream snapshot Synced; comparing the current signature against it detects
// local divergence without needing the upstream's objects locally.
type SkillMeta struct {
	Source string `toml:"source"`
	Ref    string `toml:"ref,omitempty"`
	Path   string `toml:"path,omitempty"`
	Synced string `toml:"synced,omitempty"`
	Tree   string `toml:"tree,omitempty"`
}

const manifestFileName = "skillsend.toml"

// LoadManifest reads the hub's skillsend.toml; a missing file yields an empty manifest.
func LoadManifest(hub string) (*Manifest, error) {
	m := &Manifest{Skills: map[string]SkillMeta{}}
	data, err := os.ReadFile(filepath.Join(hub, manifestFileName))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, m); err != nil {
		return nil, fmtErr("parse %s: %w", manifestFileName, err)
	}
	if m.Skills == nil {
		m.Skills = map[string]SkillMeta{}
	}
	return m, nil
}

// Save writes the manifest back to the hub root.
func (m *Manifest) Save(hub string) error {
	data, err := toml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(hub, manifestFileName), data, 0o644)
}

// Get returns the meta for a skill leaf name.
func (m *Manifest) Get(skill string) (SkillMeta, bool) {
	meta, ok := m.Skills[skill]
	return meta, ok
}

// Set records/overwrites a skill's upstream meta.
func (m *Manifest) Set(skill string, meta SkillMeta) {
	if m.Skills == nil {
		m.Skills = map[string]SkillMeta{}
	}
	m.Skills[skill] = meta
}

// Remove drops a skill's record.
func (m *Manifest) Remove(skill string) {
	delete(m.Skills, skill)
}
