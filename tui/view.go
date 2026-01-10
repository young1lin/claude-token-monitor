package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m Model) View() string {
	// Single-line mode
	if m.singleLine {
		return m.renderSingleLine()
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	// Show error if any
	if m.err != nil {
		return m.renderError()
	}

	// Show loading if not ready
	if !m.ready {
		return m.renderLoading()
	}

	// Build main UI
	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Token stats
	sections = append(sections, m.renderTokenStats())

	// Context progress bar
	sections = append(sections, m.renderContextBar())

	// Cost display
	sections = append(sections, m.renderCost())

	// Rate limit display
	sections = append(sections, m.renderRateLimit())

	// History
	if len(m.history) > 0 {
		sections = append(sections, m.renderHistory())
	}

	// Footer
	sections = append(sections, m.renderFooter())

	// Wrap in border
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Add session list sidebar if enabled
	if m.showSessionList && len(m.sessions) > 1 {
		sidebar := m.renderSessionList()
		mainContent := m.styles.Border.Render(content)
		return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", mainContent)
	}

	return m.styles.Border.Render(content)
}

// renderHeader renders the header section
func (m Model) renderHeader() string {
	left := m.styles.Title.Render("Claude Token Monitor")
	right := m.styles.Subtitle.Render(m.lastUpdate)

	// Add session info
	sessionInfo := ""
	if m.sessionID != "" {
		shortID := m.sessionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		sessionInfo = fmt.Sprintf("Session: %s", shortID)
		if m.model != "" {
			sessionInfo += fmt.Sprintf(" | Model: %s", m.model)
		}
	}

	middle := m.styles.Subtitle.Render(sessionInfo)

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", middle)
	if right != "" {
		headerRow = lipgloss.JoinHorizontal(lipgloss.Top, headerRow, "  ", right)
	}

	return headerRow
}

// renderTokenStats renders the token statistics
func (m Model) renderTokenStats() string {
	rows := []struct {
		label     string
		value     string
		highlight bool
	}{
		{"Input:", fmt.Sprintf("%s tokens", formatNumber(m.inputTokens)), false},
		{"Output:", fmt.Sprintf("%s tokens", formatNumber(m.outputTokens)), false},
		{"Cache:", fmt.Sprintf("%s tokens (discounted)", formatNumber(m.cacheTokens)), false},
		{"", "", false},
		{"Total:", fmt.Sprintf("%s tokens", formatNumber(m.totalTokens)), true},
	}

	var lines []string
	for _, row := range rows {
		if row.label == "" {
			lines = append(lines, "")
			continue
		}

		label := m.styles.Label.Render(row.label)
		var value lipgloss.Style
		if row.highlight {
			value = m.styles.Highlight
		} else {
			value = m.styles.Value
		}

		line := lipgloss.JoinHorizontal(lipgloss.Top, label, value.Render(row.value))
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderContextBar renders the context usage progress bar
func (m Model) renderContextBar() string {
	label := m.styles.Label.Render("Context:")

	// Calculate progress bar width (40 chars total)
	barWidth := 30
	fillWidth := int(m.contextPct / 100 * float64(barWidth))
	if fillWidth > barWidth {
		fillWidth = barWidth
	}

	filled := strings.Repeat("█", fillWidth)
	empty := strings.Repeat("░", barWidth-fillWidth)

	bar := m.styles.ProgressFull.Render(filled) + m.styles.ProgressEmpty.Render(empty)

	// Context text
	contextText := fmt.Sprintf(" %.1f%% / 200K", m.contextPct)
	if m.contextPct > 50 {
		contextText = m.styles.CostHigh.Render(contextText)
	} else {
		contextText = m.styles.Value.Render(contextText)
	}

	barWithText := bar + contextText

	line := lipgloss.JoinHorizontal(lipgloss.Top, label, barWithText)

	return "\n" + line
}

// renderCost renders the cost display
func (m Model) renderCost() string {
	label := m.styles.Label.Render("Cost:")
	costText := formatCost(m.cost)

	var costStyle lipgloss.Style
	if m.cost > 1.0 {
		costStyle = m.styles.CostHigh
	} else {
		costStyle = m.styles.Cost
	}

	value := costStyle.Render(costText)
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, label, value)
}

// renderRateLimit renders the rate limit status display
func (m Model) renderRateLimit() string {
	// Only show if we have rate limit data
	if m.rateLimitStatus.RequestsLimit == 0 && m.rateLimitStatus.TokensLimit == 0 {
		return ""
	}

	label := m.styles.Label.Render("Rate Limit:")

	var lines []string

	// Requests bar
	if m.rateLimitStatus.RequestsLimit > 0 {
		reqPct := m.rateLimitStatus.RequestUsage()
		reqBar := m.renderRateLimitBar(reqPct, "req", m.rateLimitStatus.RequestsRemaining, m.rateLimitStatus.RequestsLimit)
		lines = append(lines, reqBar)
	}

	// Tokens bar
	if m.rateLimitStatus.TokensLimit > 0 {
		tokenPct := m.rateLimitStatus.TokenUsage()
		tokenBar := m.renderRateLimitBar(tokenPct, "tk/min", m.rateLimitStatus.TokensRemaining, m.rateLimitStatus.TokensLimit)
		lines = append(lines, tokenBar)
	}

	result := label
	for _, line := range lines {
		result += "\n    " + line
	}

	return "\n" + result
}

// renderRateLimitBar renders a single rate limit progress bar
func (m Model) renderRateLimitBar(pct float64, label string, remaining, limit int) string {
	barWidth := 20
	fillWidth := int(pct / 100 * float64(barWidth))
	if fillWidth > barWidth {
		fillWidth = barWidth
	}

	filled := strings.Repeat("█", fillWidth)
	empty := strings.Repeat("░", barWidth-fillWidth)

	// Choose color based on status level
	var barStyle lipgloss.Style
	switch m.rateLimitStatus.GetStatusLevel() {
	case "critical":
		barStyle = m.styles.RateLimitCritical
	case "warning":
		barStyle = m.styles.RateLimitWarning
	default:
		barStyle = m.styles.RateLimitOk
	}

	bar := barStyle.Render(filled) + m.styles.ProgressEmpty.Render(empty)

	// Text info
	infoText := fmt.Sprintf(" %d/%d %s", remaining, limit, label)

	return bar + infoText
}

// renderHistory renders the history section
func (m Model) renderHistory() string {
	header := m.styles.Title.Render("\nRecent Sessions:")
	lines := []string{header}

	for i, entry := range m.history {
		if i >= 5 { // Show max 5 entries
			break
		}

		var style lipgloss.Style
		if i < 2 {
			style = m.styles.HistoryItem
		} else {
			style = m.styles.HistoryItemOld
		}

		text := fmt.Sprintf(" • %s  %s tokens  %s",
			entry.Timestamp,
			formatNumber(entry.Tokens),
			formatCost(entry.Cost),
		)

		lines = append(lines, style.Render(text))
	}

	return strings.Join(lines, "\n")
}

// renderFooter renders the footer with help text
func (m Model) renderFooter() string {
	var help string
	if len(m.sessions) > 1 {
		help = "\n\nq: quit | r: refresh | tab: switch session | f1: toggle list"
	} else {
		help = "\n\nq: quit | r: refresh"
	}
	return m.styles.Muted.Render(help)
}

// renderLoading renders the loading screen
func (m Model) renderLoading() string {
	loadingText := m.styles.Header.Render("Loading Claude Code session...")
	return m.styles.Border.Render(loadingText)
}

// renderError renders the error screen
func (m Model) renderError() string {
	errorText := m.styles.Error.Render(fmt.Sprintf("Error: %v", m.err))
	hintText := m.styles.Muted.Render("\n\nMake sure Claude Code is running and press 'r' to retry.")
	helpText := m.styles.Muted.Render("\n\nq: quit")

	content := lipgloss.JoinVertical(lipgloss.Left, errorText, hintText, helpText)
	return m.styles.Border.Render(content)
}

// formatNumber formats a number with K/M suffixes
func formatNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatCost formats a cost value in USD
func formatCost(c float64) string {
	if c >= 1.0 {
		return fmt.Sprintf("$%.2f", c)
	}
	return fmt.Sprintf("$%.4f", c)
}

// renderSingleLine renders the single-line output
func (m Model) renderSingleLine() string {
	var parts []string

	// Show loading or error
	if !m.ready {
		if m.err != nil {
			return fmt.Sprintf("Error: %v", m.err)
		}
		return "Loading..."
	}

	// Model name
	if m.model != "" {
		parts = append(parts, fmt.Sprintf("[%s]", m.model))
	}

	// Progress bar (compact)
	barWidth := 10
	fillWidth := int(m.contextPct / 100 * float64(barWidth))
	if fillWidth > barWidth {
		fillWidth = barWidth
	}

	filled := strings.Repeat("█", fillWidth)
	empty := strings.Repeat("░", barWidth-fillWidth)
	progressBar := fmt.Sprintf("[%s%s]", filled, empty)

	// Token info
	tokenInfo := fmt.Sprintf("%s/%dK (%.1f%%)",
		formatNumber(m.totalTokens),
		200, // TODO: get actual context window
		m.contextPct)

	parts = append(parts, progressBar+" "+tokenInfo)

	// Git branch
	if m.gitBranch != "" {
		gitInfo := m.gitBranch
		if m.gitStatus != "" {
			gitInfo += m.gitStatus
		}
		parts = append(parts, fmt.Sprintf("🌿 %s", gitInfo))
	}

	// Tools
	if len(m.completedTools) > 0 {
		total := 0
		for _, count := range m.completedTools {
			total += count
		}
		parts = append(parts, fmt.Sprintf("🔧 %d tools", total))
	}

	// Agent
	if len(m.agents) > 0 {
		agent := m.agents[len(m.agents)-1]
		agentInfo := agent.Type
		if agent.Desc != "" {
			desc := agent.Desc
			if len(desc) > 20 {
				desc = desc[:17] + ".."
			}
			agentInfo = fmt.Sprintf("%s: %s", agentInfo, desc)
		}
		parts = append(parts, fmt.Sprintf("🤖 %s", agentInfo))
	}

	// TODO
	if m.todoTotal > 0 {
		if m.todoCompleted == m.todoTotal {
			parts = append(parts, fmt.Sprintf("📋 ✓ %d/%d", m.todoCompleted, m.todoTotal))
		} else {
			parts = append(parts, fmt.Sprintf("📋 %d/%d", m.todoCompleted, m.todoTotal))
		}
	}

	// Cost
	if m.cost > 0 {
		parts = append(parts, fmt.Sprintf("💰 %s", formatCost(m.cost)))
	}

	return strings.Join(parts, " | ")
}

// renderSessionList renders the session list sidebar
func (m Model) renderSessionList() string {
	header := m.styles.SessionListHeader.Render("Sessions")
	lines := []string{header, ""}

	// Convert sessions map to sorted slice
	sessionList := make([]*SessionViewState, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessionList = append(sessionList, s)
	}

	// Sort by project name
	for i := 0; i < len(sessionList); i++ {
		for j := i + 1; j < len(sessionList); j++ {
			if sessionList[i].Project > sessionList[j].Project {
				sessionList[i], sessionList[j] = sessionList[j], sessionList[i]
			}
		}
	}

	for _, session := range sessionList {
		// Choose style based on whether this is the active session
		var style lipgloss.Style
		if session.SessionID == m.activeSessionID {
			style = m.styles.SessionListActive
		} else {
			style = m.styles.SessionListItem
		}

		// Format: project name with token usage
		projectName := session.Project
		if len(projectName) > 20 {
			projectName = projectName[:17] + ".."
		}

		tokenInfo := fmt.Sprintf("%sK/%dK",
			formatNumber(session.TotalTokens),
			200) // TODO: get actual context window

		line := fmt.Sprintf("▶ %s %s", projectName, tokenInfo)
		lines = append(lines, style.Render(line))
	}

	// Add hint at bottom
	lines = append(lines, "")
	lines = append(lines, m.styles.SessionListMuted.Render("Tab: switch"))

	content := strings.Join(lines, "\n")
	return m.styles.SessionListBorder.Render(content)
}
