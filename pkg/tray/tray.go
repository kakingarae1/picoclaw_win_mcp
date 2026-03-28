//go:build windows

package tray

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/getlantern/systray"
	"github.com/kakingarae1/picoclaw/pkg/config"
)

func Run(cfg *config.Config, cancel context.CancelFunc) {
	systray.Run(func() { onReady(cfg, cancel) }, func() { cancel() })
}

func onReady(cfg *config.Config, cancel context.CancelFunc) {
	systray.SetTitle("PicoClaw")
	systray.SetTooltip("PicoClaw — Windows MCP AI Assistant")
	mUI   := systray.AddMenuItem("Open Web UI", "")
	systray.AddSeparator()
	mStat := systray.AddMenuItem("Running", "")
	mStat.Disable()
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "")
	url := fmt.Sprintf("http://127.0.0.1:%d", cfg.Gateway.WebUIPort)
	go func() {
		for {
			select {
			case <-mUI.ClickedCh:
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
			case <-mQuit.ClickedCh:
				systray.Quit(); cancel(); return
			}
		}
	}()
}
