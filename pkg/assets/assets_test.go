package assets_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/assets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContent(t *testing.T) {
	// given
	content := assets.StaticContent

	// when
	entries, err := content.ReadDir(".")

	// then
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, entries[0].IsDir())
	require.Equal(t, "static", entries[0].Name())

	// when reading content of `/static`
	entries, err = content.ReadDir("static")

	// then
	require.NoError(t, err)
	require.Len(t, entries, 11)
	names := make([]string, 11)
	for i, e := range entries {
		names[i] = e.Name()
	}
	assert.ElementsMatch(t, []string{
		"OpenShift-RGB.svg", "OpenShift_Data_Science-RGB.svg",
		"codereadyws-logo.svg", "index.html", "landingpage.js",
		"redhat-logo.svg", "silent-check-sso.html", "favicon.ico",
		"landingpage.css", "openshift-logo.svg", "rhdeveloper-logo.svg",
	}, names)
}
