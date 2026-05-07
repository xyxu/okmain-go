package main

import (
	"fmt"
	"os"
	"path/filepath"

	okmain "github.com/xyxu/okmain-go"
)

func main() {
	if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: check <image> [<image>...]")
			os.Exit(1)
	}
	for _, path := range os.Args[1:] {
		input, err := okmain.NewInputImageFromFile(path)
		if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
				os.Exit(1)
		}
		colors := okmain.Colors(input)
		fmt.Printf("%s ", filepath.Base(path))
		for i, c := range colors {
				if i > 0 {
						fmt.Print(" ")
				}
				fmt.Print(c.Hex())
		}
		fmt.Println()
	}
}
