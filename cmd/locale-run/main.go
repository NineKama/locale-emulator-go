package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"locale-emulator-go/internal/launcher"
)

func main() {
	dll := flag.String("engine", "", "override engine DLL (default: select matching bundled engine)")
	jsonOutput := flag.Bool("json", false, "machine-readable launch result")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: locale-run [-engine path] target.exe")
		os.Exit(2)
	}
	target, e := filepath.Abs(flag.Arg(0))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	result, e := launcher.Start(target, *dll)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	if *jsonOutput {
		json.NewEncoder(os.Stdout).Encode(result)
		return
	}
	fmt.Printf("Hook installed; launched PID %d (%s / ja-JP / CP932)\n", result.PID, result.Architecture)
}
