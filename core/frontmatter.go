package core

import (
	"os"
	"path/filepath"
	"strings"
)

// ParseFrontmatter extracts the name and description fields from SKILL.md content.
func ParseFrontmatter(data string) (name, description string) {
	meta, _ := splitFrontmatter(data)
	return meta["name"], meta["description"]
}

// ReadSkillMD parses a skill's SKILL.md directory: frontmatter name,
// description, and the trimmed markdown body.
func ReadSkillMD(dir string) (fmName, description, body string) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return "", "", ""
	}
	meta, b := splitFrontmatter(string(data))
	return meta["name"], strings.TrimSpace(meta["description"]), strings.TrimSpace(b)
}

// readSkillMeta reads a skill's frontmatter name and description from its SKILL.md.
func readSkillMeta(skillDir string) (fmName, description string) {
	n, d, _ := ReadSkillMD(skillDir)
	return n, d
}
