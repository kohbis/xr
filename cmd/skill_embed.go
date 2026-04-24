package cmd

var skillMDEmbedded string

// SetEmbeddedSkillMD sets the embedded SKILL.md contents.
// It is intended to be called from main (package main) which can embed files in the repo root.
func SetEmbeddedSkillMD(md string) {
	skillMDEmbedded = md
}

