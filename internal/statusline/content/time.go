package content

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Usage cache
var (
	usageCache     *UsageData
	usageCacheMu   sync.RWMutex
	usageCacheTime time.Time
	usageCacheTTL  = 5 * time.Minute
)

// CredentialsFile represents the Claude credentials file
type CredentialsFile struct {
	ClaudeAiOauth *struct {
		AccessToken      string `json:"accessToken"`
		SubscriptionType string `json:"subscriptionType"`
		ExpiresAt        int64  `json:"expiresAt"`
	} `json:"claudeAiOauth"`
}

// UsageApiResponse represents the OAuth usage API response
type UsageApiResponse struct {
	FiveHour *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
}

// UsageData holds parsed usage information
type UsageData struct {
	FiveHour       float64
	SevenDay       float64
	FiveHourResetAt   time.Time
	SevenDayResetAt   time.Time
	APIUnavailable bool
}

// CurrentTimeCollector collects the current time
type CurrentTimeCollector struct {
	*BaseCollector
}

// NewCurrentTimeCollector creates a new current time collector
func NewCurrentTimeCollector() *CurrentTimeCollector {
	return &CurrentTimeCollector{
		BaseCollector: NewBaseCollector(ContentCurrentTime, 1*time.Second, false),
	}
}

// Collect returns the current time
func (c *CurrentTimeCollector) Collect(input interface{}, summary interface{}) (string, error) {
	return fmt.Sprintf("🕐 %s", time.Now().Format("2006-01-02 15:04")), nil
}

// QuotaCollector collects subscription quota usage
type QuotaCollector struct {
	*BaseCollector
}

// NewQuotaCollector creates a new quota collector
func NewQuotaCollector() *QuotaCollector {
	return &QuotaCollector{
		BaseCollector: NewBaseCollector(ContentQuota, 5*time.Minute, true),
	}
}

// Collect returns subscription quota usage
func (c *QuotaCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	return getSubscriptionQuota(statusInput), nil
}

// getSubscriptionQuota returns the subscription quota usage percentage with reset time
func getSubscriptionQuota(input *StatusLineInput) string {
	usage := getSubscriptionUsage()

	if usage == nil {
		return ""
	}

	if usage.FiveHour > 0 {
		resetTime := formatResetTime(usage.FiveHourResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% · Reset %s", usage.FiveHour, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%%", usage.FiveHour)
	}

	if usage.SevenDay > 0 {
		resetTime := formatResetTime(usage.SevenDayResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% · Reset %s", usage.SevenDay, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%%", usage.SevenDay)
	}

	return ""
}

// formatResetTime formats the reset time in local timezone with timezone name
func formatResetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	local := t.Local()
	timeStr := local.Format("15:04")
	zoneName := getLocalTimeZoneName()
	return fmt.Sprintf("%s (%s)", timeStr, zoneName)
}

// getLocalTimeZoneName attempts to get the IANA timezone name
func getLocalTimeZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}

	if linkTarget, err := os.Readlink("/etc/localtime"); err == nil {
		if idx := strings.LastIndex(linkTarget, "zoneinfo/"); idx >= 0 {
			return linkTarget[idx+9:]
		}
	}

	_, zoneOffset := time.Now().Zone()
	if zoneOffset == 0 {
		return "UTC"
	}

	sign := "+"
	if zoneOffset < 0 {
		sign = "-"
		zoneOffset = -zoneOffset
	}
	zoneHours := zoneOffset / 3600
	zoneMinutes := (zoneOffset % 3600) / 60

	if zoneMinutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, zoneHours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, zoneHours, zoneMinutes)
}

// getSubscriptionUsage fetches subscription usage from Claude OAuth API
func getSubscriptionUsage() *UsageData {
	now := time.Now()

	usageCacheMu.RLock()
	if usageCache != nil && now.Sub(usageCacheTime) < usageCacheTTL {
		cached := *usageCache
		usageCacheMu.RUnlock()
		return &cached
	}
	usageCacheMu.RUnlock()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	credData, err := os.ReadFile(credPath)
	if err != nil {
		return nil
	}

	var creds CredentialsFile
	if err := json.Unmarshal(credData, &creds); err != nil {
		return nil
	}

	if creds.ClaudeAiOauth == nil || creds.ClaudeAiOauth.AccessToken == "" {
		return nil
	}

	if creds.ClaudeAiOauth.ExpiresAt > 0 && creds.ClaudeAiOauth.ExpiresAt < now.UnixMilli() {
		return nil
	}

	subType := strings.ToLower(creds.ClaudeAiOauth.SubscriptionType)
	if subType == "" || strings.Contains(subType, "api") {
		return nil
	}

	usage, err := fetchUsageAPI(creds.ClaudeAiOauth.AccessToken)
	if err != nil || usage == nil {
		usageCacheMu.Lock()
		usageCache = &UsageData{APIUnavailable: true}
		usageCacheTime = now
		usageCacheMu.Unlock()
		return nil
	}

	usageCacheMu.Lock()
	usageCache = usage
	usageCacheTime = now
	usageCacheMu.Unlock()

	return usage
}

// fetchUsageAPI calls the Claude OAuth usage API
func fetchUsageAPI(accessToken string) (*UsageData, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-token-monitor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp UsageApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	usage := &UsageData{}

	if apiResp.FiveHour != nil {
		usage.FiveHour = apiResp.FiveHour.Utilization
		if apiResp.FiveHour.ResetsAt != "" {
			usage.FiveHourResetAt, _ = time.Parse(time.RFC3339, apiResp.FiveHour.ResetsAt)
		}
	}

	if apiResp.SevenDay != nil {
		usage.SevenDay = apiResp.SevenDay.Utilization
		if apiResp.SevenDay.ResetsAt != "" {
			usage.SevenDayResetAt, _ = time.Parse(time.RFC3339, apiResp.SevenDay.ResetsAt)
		}
	}

	return usage, nil
}
