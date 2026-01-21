// Package composers provides built-in content composers for the statusline
package composers

import (
	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
)

// TokenComposer combines model, token bar, and token info into a single display
// Default format: [model token-bar token-info]
type TokenComposer struct {
	composer content.Composer
}

// NewTokenComposer creates a new token composer with the default format
func NewTokenComposer() *TokenComposer {
	// Use FormatComposer for the complex logic
	return &TokenComposer{
		composer: content.NewFormatComposer("token", []content.ContentType{
			content.ContentModel,
			content.ContentTokenBar,
			content.ContentTokenInfo,
		}, func(contents map[content.ContentType]string) string {
			model := contents[content.ContentModel]
			bar := contents[content.ContentTokenBar]
			info := contents[content.ContentTokenInfo]

			line := model
			if line == "" {
				line = "Claude"
			}
			if bar != "" {
				line += " " + bar
			}
			if info != "" {
				line += " " + info
			}
			return "[" + line + "]"
		}),
	}
}

// NewTokenComposerSimple creates a simple token composer (model + token-bar only)
func NewTokenComposerSimple() *TokenComposer {
	return &TokenComposer{
		composer: content.NewFormatComposer("token-simple", []content.ContentType{
			content.ContentModel,
			content.ContentTokenBar,
		}, func(contents map[content.ContentType]string) string {
			model := contents[content.ContentModel]
			bar := contents[content.ContentTokenBar]

			line := model
			if line == "" {
				line = "Claude"
			}
			if bar != "" {
				line += " " + bar
			}
			return "[" + line + "]"
		}),
	}
}

// NewTokenComposerModelOnly creates a token composer that only shows the model
func NewTokenComposerModelOnly() *TokenComposer {
	return &TokenComposer{
		composer: content.NewFormatComposer("token-model-only", []content.ContentType{
			content.ContentModel,
		}, func(contents map[content.ContentType]string) string {
			model := contents[content.ContentModel]
			if model == "" {
				model = "Claude"
			}
			return "[" + model + "]"
		}),
	}
}

// Name returns the composer's name
func (c *TokenComposer) Name() string {
	return c.composer.Name()
}

// InputTypes returns the content types this composer consumes
func (c *TokenComposer) InputTypes() []content.ContentType {
	return c.composer.InputTypes()
}

// Compose combines the token-related contents
func (c *TokenComposer) Compose(contents map[content.ContentType]string) string {
	return c.composer.Compose(contents)
}

// TokenComposerConfig represents configuration for a custom token composer
type TokenComposerConfig struct {
	Name        string
	ShowBar     bool
	ShowInfo    bool
	Prefix      string
	Suffix      string
	ModelPrefix string // E.g., "🤖 "
}

// NewTokenComposerFromConfig creates a token composer from configuration
func NewTokenComposerFromConfig(cfg TokenComposerConfig) *TokenComposer {
	inputTypes := []content.ContentType{content.ContentModel}
	if cfg.ShowBar {
		inputTypes = append(inputTypes, content.ContentTokenBar)
	}
	if cfg.ShowInfo {
		inputTypes = append(inputTypes, content.ContentTokenInfo)
	}

	return &TokenComposer{
		composer: content.NewFormatComposer(cfg.Name, inputTypes, func(contents map[content.ContentType]string) string {
			model := contents[content.ContentModel]
			if model == "" {
				model = "Claude"
			}
			if cfg.ModelPrefix != "" {
				model = cfg.ModelPrefix + model
			}

			line := model
			if cfg.ShowBar {
				if bar := contents[content.ContentTokenBar]; bar != "" {
					line += " " + bar
				}
			}
			if cfg.ShowInfo {
				if info := contents[content.ContentTokenInfo]; info != "" {
					line += " " + info
				}
			}
			return cfg.Prefix + line + cfg.Suffix
		}),
	}
}
