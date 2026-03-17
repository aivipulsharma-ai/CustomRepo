package main

import (
	"fmt"
	"os"

	"github.com/dextr_avs/cmd"
)

// Run runs the Dextr AVS application
func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
