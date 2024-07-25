package context

import (
	"github.com/labstack/echo/v4"
)

// IsPublicViewerEnabled retrieves from the context the boolean value associated to the PublicViewerEnabled key.
// If the key is not set it returns false, otherwise it returns the boolean value stored in the context.
func IsPublicViewerEnabled(ctx echo.Context) bool {
	publicViewerEnabled, _ := ctx.Get(PublicViewerEnabled).(bool)
	return publicViewerEnabled
}
