package commands

import "fmt"

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
		fmt.Println("dev")
		return nil
	}
	fmt.Println("version:", Version)
	fmt.Println("commit:", Commit)

	return nil
}
