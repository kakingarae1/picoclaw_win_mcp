package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kakingarae1/picoclaw/pkg/secrets"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: keytool <set|get|delete> <provider> [key]")
		os.Exit(1)
	}
	cmd, provider := os.Args[1], os.Args[2]
	switch cmd {
	case "set":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: keytool set <provider> <key>")
			os.Exit(1)
		}
		if err := secrets.Set(provider, os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("stored key for %q in Windows Credential Manager\n", provider)
	case "get":
		key, err := secrets.Get(provider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(key) > 8 {
			key = key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
		}
		fmt.Printf("key for %q: %s\n", provider, key)
	case "delete":
		if err := secrets.Delete(provider); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("deleted key for %q\n", provider)
	default:
		fmt.Fprintf(os.Stderr, "Unknown: %s\n", cmd)
		os.Exit(1)
	}
}
