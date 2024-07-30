package context_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestIsPublicViewerEnabled(t *testing.T) {
	tt := map[string]struct {
		data     map[string]interface{}
		expected bool
	}{
		"context value is true": {
			data:     map[string]interface{}{context.PublicViewerEnabled: true},
			expected: true,
		},
		"context value is false": {
			data:     map[string]interface{}{context.PublicViewerEnabled: false},
			expected: false,
		},
		"value not set in context": {
			data:     map[string]interface{}{},
			expected: false,
		},
		"value set to a not castable value": {
			data:     map[string]interface{}{context.PublicViewerEnabled: struct{}{}},
			expected: false,
		},
		"value set to nil": {
			data:     map[string]interface{}{context.PublicViewerEnabled: nil},
			expected: false,
		},
	}

	for k, tc := range tt {
		t.Run(k, func(t *testing.T) {
			// given
			ctx := echo.New().NewContext(nil, nil)
			for k, v := range tc.data {
				ctx.Set(k, v)
			}

			// when
			actual := context.IsPublicViewerEnabled(ctx)

			// then
			require.Equal(t, tc.expected, actual, "IsPublicViewerEnabled returned a value different from expected")
		})
	}
}
