package main

import (
	"os"
	"testing"
)

func TestMainFunction(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"mpvt", "version"}
	main()
}
