package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheckHandler(t *testing.T) {

	t.Run("service creation production mode", func(t *testing.T) {
		s := NewHealthService(false)
		assert.False(t, s.isTestMode, "Test mode not set correctly")
	})

	t.Run("service creation test mode", func(t *testing.T) {
		s := NewHealthService(true)
		assert.True(t, s.isTestMode, "Test mode not set correctly")
	})

	t.Run("service reply test mode", func(t *testing.T) {
		s := NewHealthService(true)
		response := s.getHealthInfo()
		val, ok := response["alive"]
		assert.True(t, ok, "no alive key in health response")
		assert.True(t, val, "alive is false in test mode health response")
	})

	t.Run("service reply production mode", func(t *testing.T) {
		s := NewHealthService(false)
		response := s.getHealthInfo()
		val, ok := response["alive"]
		assert.True(t, ok, "no alive key in health response")
		assert.True(t, val, "alive is false in production mode health response")
	})

}
