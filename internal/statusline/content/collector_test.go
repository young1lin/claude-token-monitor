package content

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBaseCollector(t *testing.T) {
	tests := []struct {
		name        string
		contentType ContentType
		ttl         time.Duration
		optional    bool
	}{
		{
			name:        "non-optional collector with 5s TTL",
			contentType: ContentModel,
			ttl:         5 * time.Second,
			optional:    false,
		},
		{
			name:        "optional collector with 60s TTL",
			contentType: ContentAgent,
			ttl:         60 * time.Second,
			optional:    true,
		},
		{
			name:        "zero TTL",
			contentType: ContentFolder,
			ttl:         0,
			optional:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			c := NewBaseCollector(tt.contentType, tt.ttl, tt.optional)

			// Assert
			require.NotNil(t, c)
			assert.Equal(t, tt.contentType, c.contentType)
			assert.Equal(t, tt.ttl, c.cacheTTL)
			assert.Equal(t, tt.optional, c.optional)
		})
	}
}

func TestBaseCollector_Type(t *testing.T) {
	// Arrange
	c := NewBaseCollector(ContentTokenBar, 10*time.Second, false)

	// Act
	typ := c.Type()

	// Assert
	assert.Equal(t, ContentTokenBar, typ)
}

func TestBaseCollector_CacheTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{name: "1 second", ttl: 1 * time.Second},
		{name: "5 minutes", ttl: 5 * time.Minute},
		{name: "zero", ttl: 0},
		{name: "1 hour", ttl: 1 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			c := NewBaseCollector(ContentModel, tt.ttl, false)

			// Act
			got := c.CacheTTL()

			// Assert
			assert.Equal(t, tt.ttl, got)
		})
	}
}

func TestBaseCollector_Optional(t *testing.T) {
	tests := []struct {
		name     string
		optional bool
	}{
		{name: "optional", optional: true},
		{name: "non-optional", optional: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			c := NewBaseCollector(ContentSkills, 60*time.Second, tt.optional)

			// Act
			got := c.Optional()

			// Assert
			assert.Equal(t, tt.optional, got)
		})
	}
}

func TestCachedContent_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "expired content (past expiry)",
			expiresAt: time.Now().Add(-1 * time.Second),
			want:      true,
		},
		{
			name:      "non-expired content (far future)",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "just expired content",
			expiresAt: time.Now().Add(-1 * time.Nanosecond),
			want:      true,
		},
		{
			name:      "expires exactly now (edge case, technically not expired)",
			expiresAt: time.Now().Add(100 * time.Millisecond),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cached := &cachedContent{
				value:     "test-value",
				expiresAt: tt.expiresAt,
			}

			// Act
			got := cached.isExpired()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
