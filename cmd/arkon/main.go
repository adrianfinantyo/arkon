package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Printf("arkon %s (commit: %s, built: %s)\n", version, commit, date)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Arkon - Lightweight Host Lifecycle & Docker Stack Controller

Usage:
  arkon [command] [flags]

Available Commands:
  apply      Compile arkon.yaml and sync host systemd units
  status     Display host power state, RTC alarm, and timer status
  sleep      Execute graceful profile teardown and program RTC wakealarm
  inhibit    Temporarily pause automated scheduled shutdown
  version    Print Arkon version information
  help       Show command help

Flags:
  -c, --config string   Path to arkon.yaml manifest (default: "./arkon.yaml")
  -h, --help            Show help for command`)
}
