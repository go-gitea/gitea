package commands

import (
	"fmt"
	"runtime/debug"
)

var (
	// Version for the swagger command
	Version string
	// Commit for the swagger command
	Commit string
)

// PrintVersion the command
type PrintVersion struct {
}

// Execute this command
func (p *PrintVersion) Execute(args []string) error {
	if Version == "" {
		if info, available := debug.ReadBuildInfo(); available && info.Main.Version != "(devel)" {
			// built from source, with module (e.g. go get)
			fmt.Println("version:", info.Main.Version)
			fmt.Println("commit:", fmt.Sprintf("(unknown, mod sum: %q)", info.Main.Sum))
			return nil
		}
		// built from source, local repo
		fmt.Println("dev")
		return nil
	}
	// released version
	fmt.Println("version:", Version)
	fmt.Println("commit:", Commit)

	return nil
}
