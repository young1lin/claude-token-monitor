package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/young1lin/claude-token-monitor/internal/config"
	"github.com/young1lin/claude-token-monitor/internal/monitor"
	"github.com/young1lin/claude-token-monitor/internal/store"
	"github.com/young1lin/claude-token-monitor/internal/update"
	"github.com/young1lin/claude-token-monitor/tui"
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
		ProjectsDir:   config.ProjectsDir(),
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return monitor.FindActiveSession(config.ProjectsDir())
		},
		ProjectDiscoverer: func() ([]tui.ProjectInfo, error) {
			result, err := monitor.DiscoverProjects(monitor.DiscoverConfig{
				ProjectsDir: config.ProjectsDir(),
			})
			if err != nil {
				return nil, err
			}

			// Convert monitor.ProjectInfo to tui.ProjectInfo
			projects := make([]tui.ProjectInfo, 0, len(result.Projects))
			for _, p := range result.Projects {
				projects = append(projects, tui.ProjectInfo{
					Name:              p.Name,
					SessionCount:      p.SessionCount,
					LastActivity:      p.LastActivity,
					MostRecentSession: p.MostRecentSession,
				})
			}

			return projects, nil
		},
		DBOpener: store.Open,
		WatcherCreator: func(path string) (monitor.WatcherInterface, error) {
			return monitor.NewWatcher(path)
		},
		ProgramRunner: func(p *tea.Program) error {
			_, err := p.Run()
			return err
		},
		SingleLine: false, // TUI 模式（显示项目选择）
	}); err != nil {
		logAndExit(err)
	}
}

func logAndExit(err error) {
	// This is a separate function to allow testing of error handling
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}
