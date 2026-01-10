package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/young1lin/claude-token-monitor/internal/config"
	"github.com/young1lin/claude-token-monitor/internal/monitor"
	"github.com/young1lin/claude-token-monitor/internal/store"
	"github.com/young1lin/claude-token-monitor/internal/update"
)

// exitFunc is the function to call for exiting (can be mocked for testing)
var exitFunc = os.Exit

func main() {
	// 异步检查更新
	go func() {
		checker := update.NewChecker(update.Version)
		release, err := checker.Check()
		if err != nil {
			// 静默失败，不影响主程序
			return
		}
		if release != nil {
			// TODO: 可以通过 channel 或其他方式通知 TUI
			// 目前仅打印到 stderr
			fmt.Fprintf(os.Stderr, "\n🎉 Update available: %s → %s\n",
				update.Version, release.TagName)
			fmt.Fprintf(os.Stderr, "Visit %s to download\n\n", release.HTMLURL)
		}
	}()

	if err := run(&AppDependencies{
		ProjectsDir:    config.ProjectsDir(),
		SessionFinder:  monitor.FindCurrentSession,
		DBOpener:       store.Open,
		WatcherCreator: func(path string) (monitor.WatcherInterface, error) {
			return monitor.NewWatcher(path)
		},
		ProgramRunner: func(p *tea.Program) error {
			_, err := p.Run()
			return err
		},
		SingleLine:     true, // 默认单行模式
	}); err != nil {
		logAndExit(err)
	}
}

func logAndExit(err error) {
	// This is a separate function to allow testing of error handling
	if err != nil {
		exitFunc(1)
	}
}
