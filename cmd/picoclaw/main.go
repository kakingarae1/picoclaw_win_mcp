package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kakingarae1/picoclaw/pkg/agent"
	"github.com/kakingarae1/picoclaw/pkg/config"
	"github.com/kakingarae1/picoclaw/pkg/gateway"
	"github.com/kakingarae1/picoclaw/pkg/tray"
)

func main() {
	if len(os.Args) < 2 {
		runAll()
		return
	}
	switch os.Args[1] {
	case "agent":
		runCLI()
	case "gateway":
		runGateway()
	case "tray":
		runTray()
	case "onboard":
		runOnboard()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runAll() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gw := gateway.New(cfg)
	go func() {
		if err := gw.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Gateway error: %v\n", err)
		}
	}()
	tray.Run(cfg, cancel)
}

func runCLI() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	msg := ""
	if len(os.Args) > 3 && os.Args[2] == "-m" {
		msg = os.Args[3]
	}
	a := agent.New(cfg)
	if msg != "" {
		resp, err := a.Chat(ctx, msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)
	} else {
		a.Interactive(ctx)
	}
}

func runGateway() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := gateway.New(cfg).Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Gateway error: %v\n", err)
		os.Exit(1)
	}
}

func runTray() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}
	_, cancel := context.WithCancel(context.Background())
	tray.Run(cfg, cancel)
}

func runOnboard() {
	if err := config.Onboard(); err != nil {
		fmt.Fprintf(os.Stderr, "Onboard error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done. Edit %APPDATA%\\picoclaw\\config.yaml then run picoclaw.exe")
}
