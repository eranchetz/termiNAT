package main

import (
	"github.com/doitintl/terminator/cmd"
)

// Version is set via ldflags during build
var Version = "dev"

func main() {
	cmd.Execute()
}
