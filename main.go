package main

import (
	_ "embed"

	"github.com/kohbis/xr/cmd"
)

// Set at link time by GoReleaser: -X main.version={{.Version}}
var version = "dev"

//go:embed SKILL.md
var embeddedSkillMD string

func main() {
	cmd.SetVersion(version)
	cmd.SetEmbeddedSkillMD(embeddedSkillMD)
	cmd.Execute()
}
