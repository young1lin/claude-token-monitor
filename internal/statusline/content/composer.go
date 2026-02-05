// Package content provides content composition for the statusline
// Content Layer: Data composition from multiple content types
package content

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Composer combines multiple content types into a single output
type Composer interface {
	// Name returns the unique identifier for this composer
	Name() string

	// InputTypes returns the content types this composer consumes
	InputTypes() []ContentType

	// Compose combines the input contents into a single string
	Compose(contents map[ContentType]string) string
}

// BaseComposer provides a template-based composer implementation
type BaseComposer struct {
	name     string
	inputTypes []ContentType
	template string
	parsed   *template.Template // Pre-parsed template for performance
}

// NewBaseComposer creates a new template-based composer with pre-parsed template
func NewBaseComposer(name string, inputTypes []ContentType, tmpl string) *BaseComposer {
	var parsed *template.Template
	if tmpl != "" {
		// Pre-parse the template once during construction
		parsed, _ = template.New(name).Option("missingkey=zero").Parse(tmpl)
	}
	return &BaseComposer{
		name:       name,
		inputTypes: inputTypes,
		template:   tmpl,
		parsed:     parsed,
	}
}

// Name returns the composer's name
func (c *BaseComposer) Name() string {
	return c.name
}

// InputTypes returns the content types this composer consumes
func (c *BaseComposer) InputTypes() []ContentType {
	return c.inputTypes
}

// Compose executes the template with the provided contents
func (c *BaseComposer) Compose(contents map[ContentType]string) string {
	if c.template == "" {
		return ""
	}

	// Build template data map with proper Go template key format
	// Go templates require keys to be valid identifiers (no hyphens)
	data := make(map[string]interface{})
	for _, ct := range c.inputTypes {
		// Use the raw string as key, access via .key syntax
		data[string(ct)] = contents[ct]
	}

	// Use pre-parsed template if available, otherwise fall back to runtime parsing
	tmpl := c.parsed
	if tmpl == nil {
		// Runtime parse if pre-parsing failed (shouldn't happen)
		var err error
		tmpl, err = template.New(c.name).Option("missingkey=zero").Parse(c.template)
		if err != nil {
			return c.fallbackCompose(contents)
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return c.fallbackCompose(contents)
	}

	return buf.String()
}

// fallbackCompose provides simple concatenation when template fails
func (c *BaseComposer) fallbackCompose(contents map[ContentType]string) string {
	var parts []string
	for _, ct := range c.inputTypes {
		if val := contents[ct]; val != "" {
			parts = append(parts, val)
		}
	}
	return strings.Join(parts, " ")
}

// SimpleComposer creates a composer that joins contents with a separator
type SimpleComposer struct {
	name       string
	inputTypes []ContentType
	separator  string
	prefix     string
	suffix     string
}

// NewSimpleComposer creates a new simple composer
func NewSimpleComposer(name string, inputTypes []ContentType, separator, prefix, suffix string) *SimpleComposer {
	return &SimpleComposer{
		name:       name,
		inputTypes: inputTypes,
		separator:  separator,
		prefix:     prefix,
		suffix:     suffix,
	}
}

// Name returns the composer's name
func (c *SimpleComposer) Name() string {
	return c.name
}

// InputTypes returns the content types this composer consumes
func (c *SimpleComposer) InputTypes() []ContentType {
	return c.inputTypes
}

// Compose joins non-empty contents with the separator
func (c *SimpleComposer) Compose(contents map[ContentType]string) string {
	var parts []string
	for _, ct := range c.inputTypes {
		if val := contents[ct]; val != "" {
			parts = append(parts, val)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	result := strings.Join(parts, c.separator)
	if c.prefix != "" {
		result = c.prefix + result
	}
	if c.suffix != "" {
		result = result + c.suffix
	}
	return result
}

// FormatComposer creates a composer with a custom format function
type FormatComposer struct {
	name       string
	inputTypes []ContentType
	formatFunc func(map[ContentType]string) string
}

// NewFormatComposer creates a new format composer with a custom function
func NewFormatComposer(name string, inputTypes []ContentType, formatFunc func(map[ContentType]string) string) *FormatComposer {
	return &FormatComposer{
		name:       name,
		inputTypes: inputTypes,
		formatFunc: formatFunc,
	}
}

// Name returns the composer's name
func (c *FormatComposer) Name() string {
	return c.name
}

// InputTypes returns the content types this composer consumes
func (c *FormatComposer) InputTypes() []ContentType {
	return c.inputTypes
}

// Compose calls the custom format function
func (c *FormatComposer) Compose(contents map[ContentType]string) string {
	if c.formatFunc == nil {
		return ""
	}
	return c.formatFunc(contents)
}

// ConditionalComposer conditionally formats based on which contents are present
type ConditionalComposer struct {
	name           string
	inputTypes     []ContentType
	formatPatterns []ConditionalPattern
}

// ConditionalPattern defines a format pattern with conditions
type ConditionalPattern struct {
	// Required specifies which content types must be present
	Required []ContentType
	// Optional specifies which content types may be present
	Optional []ContentType
	// Format is the template to use (empty = skip this pattern)
	Format string
	// Parsed is the pre-parsed template for performance
	Parsed *template.Template
}

// NewConditionalComposer creates a new conditional composer with pre-parsed patterns
func NewConditionalComposer(name string, inputTypes []ContentType, patterns []ConditionalPattern) *ConditionalComposer {
	// Pre-parse all patterns for performance
	for i := range patterns {
		if patterns[i].Format != "" {
			patterns[i].Parsed, _ = template.New(name).Option("missingkey=zero").Parse(patterns[i].Format)
		}
	}
	return &ConditionalComposer{
		name:           name,
		inputTypes:     inputTypes,
		formatPatterns: patterns,
	}
}

// Name returns the composer's name
func (c *ConditionalComposer) Name() string {
	return c.name
}

// InputTypes returns the content types this composer consumes
func (c *ConditionalComposer) InputTypes() []ContentType {
	return c.inputTypes
}

// Compose finds the first matching pattern and formats it
func (c *ConditionalComposer) Compose(contents map[ContentType]string) string {
	for _, pattern := range c.formatPatterns {
		if c.matchesPattern(contents, pattern) {
			if pattern.Format == "" {
				continue
			}

			// Use pre-parsed template if available
			tmpl := pattern.Parsed
			if tmpl == nil {
				// Runtime parse if pre-parsing failed
				var err error
				tmpl, err = template.New(c.name).Option("missingkey=zero").Parse(pattern.Format)
				if err != nil {
					continue
				}
			}

			// Build template data
			data := make(map[string]interface{})
			allTypes := append(pattern.Required, pattern.Optional...)
			for _, ct := range allTypes {
				data[string(ct)] = contents[ct]
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err == nil {
				return buf.String()
			}
		}
	}

	// No pattern matched, return empty
	return ""
}

// matchesPattern checks if the contents match a pattern
func (c *ConditionalComposer) matchesPattern(contents map[ContentType]string, pattern ConditionalPattern) bool {
	// Check required fields are present and non-empty
	for _, ct := range pattern.Required {
		if contents[ct] == "" {
			return false
		}
	}
	return true
}

// PassthroughComposer returns the first non-empty content as-is
type PassthroughComposer struct {
	name       string
	inputTypes []ContentType
}

// NewPassthroughComposer creates a new passthrough composer
func NewPassthroughComposer(name string, inputTypes []ContentType) *PassthroughComposer {
	return &PassthroughComposer{
		name:       name,
		inputTypes: inputTypes,
	}
}

// Name returns the composer's name
func (c *PassthroughComposer) Name() string {
	return c.name
}

// InputTypes returns the content types this composer consumes
func (c *PassthroughComposer) InputTypes() []ContentType {
	return c.inputTypes
}

// Compose returns the first non-empty content
func (c *PassthroughComposer) Compose(contents map[ContentType]string) string {
	for _, ct := range c.inputTypes {
		if val := contents[ct]; val != "" {
			return val
		}
	}
	return ""
}

// Registry holds all registered composers
type Registry struct {
	composers map[string]Composer
}

// NewRegistry creates a new composer registry
func NewRegistry() *Registry {
	return &Registry{
		composers: make(map[string]Composer),
	}
}

// Register adds a composer to the registry
func (r *Registry) Register(composer Composer) {
	r.composers[composer.Name()] = composer
}

// Get retrieves a composer by name
func (r *Registry) Get(name string) (Composer, bool) {
	c, ok := r.composers[name]
	return c, ok
}

// List returns all registered composer names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.composers))
	for name := range r.composers {
		names = append(names, name)
	}
	return names
}

// MustGet retrieves a composer by name or panics
func (r *Registry) MustGet(name string) Composer {
	c, ok := r.Get(name)
	if !ok {
		panic(fmt.Sprintf("composer not found: %s", name))
	}
	return c
}
