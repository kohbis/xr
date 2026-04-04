package main

import "github.com/kohbis/xr/cmd"

// Set at link time by GoReleaser: -X main.version={{.Version}}
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
