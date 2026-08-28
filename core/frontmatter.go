package core

import (
	"os"
	"path/filepath"
	"strings"
)

// ParseFrontmatter extracts name/description/title from SKILL.md content.
func ParseFrontmatter(data string) (name, description string) {
	meta, _ := splitFrontmatter(data)
	return meta["name"], meta["description"]
}

// readDescription reads a skill's description from its SKILL.md.
func readDescription(skillDir string) string {
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	_, description := ParseFrontmatter(string(data))
	return strings.TrimSpace(description)
}
