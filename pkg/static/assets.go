// +build dev

package static

import "net/http"

// Assets contains project assets.
var Assets http.FileSystem = http.Dir("pkg/assets")
