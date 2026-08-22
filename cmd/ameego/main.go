package main

import (
	"os"

	"github.com/yurimoinhos/ameego/internal/cliapp"
)

func main() {
	os.Exit(cliapp.Run(os.Args))
}
