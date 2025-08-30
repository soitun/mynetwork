package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mattn/go-isatty"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func getDefaultConfigPath(ifName string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "mynetwork", ifName+".json")
	}
	return "/etc/mynetwork/" + ifName + ".json"
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func printListF(strings []string, handler func(string) string) {
	for _, s := range strings {
		fmt.Printf("    %s\n", handler(s))
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func isTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
