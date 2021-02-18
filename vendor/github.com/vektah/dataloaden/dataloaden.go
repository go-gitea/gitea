package main

import (
	"fmt"
	"os"

	"github.com/vektah/dataloaden/pkg/generator"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("usage: name keyType valueType")
		fmt.Println(" example:")
		fmt.Println(" dataloaden 'UserLoader int []*github.com/my/package.User'")
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	if err := generator.Generate(os.Args[1], os.Args[2], os.Args[3], wd); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
}
