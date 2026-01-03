package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetIntQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		queryKey     string
		queryValue   string
		defaultValue int
		want         int
	}{
		{
			name:         "valid integer",
			queryKey:     "duration_ms",
			queryValue:   "1000",
			defaultValue: 0,
			want:         1000,
		},
		{
			name:         "empty value",
			queryKey:     "duration_ms",
			queryValue:   "",
			defaultValue: 500,
			want:         500,
		},
		{
			name:         "invalid integer",
			queryKey:     "duration_ms",
			queryValue:   "abc",
			defaultValue: 100,
			want:         100,
		},
		{
			name:         "missing key",
			queryKey:     "other_key",
			queryValue:   "1000",
			defaultValue: 200,
			want:         200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			if tt.queryValue != "" || tt.name == "invalid integer" {
				c.Request = httptest.NewRequest("GET", "/test?"+tt.queryKey+"="+tt.queryValue, nil)
			} else {
				c.Request = httptest.NewRequest("GET", "/test", nil)
			}

			got := GetIntQuery(c, "duration_ms", tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}
