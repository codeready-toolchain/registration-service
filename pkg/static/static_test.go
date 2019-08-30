package static_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codeready-toolchain/registration-service/pkg/static"
)

func TestStatic(t *testing.T) {
	// get the static assets.
	hfs := static.Assets
	// open the default file; note that the
	// actual files and contents are tested elsewhere.
	file, err := hfs.Open("index.html")
	require.NoError(t, err)
	// check the file stats
	stat, err := file.Stat()
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0), "static asset 'index.html' size is zero.")
}
