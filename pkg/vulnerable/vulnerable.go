// pkg/vulnerable/command_injection.go
package vulnerable

import (
	"os/exec"

	"github.com/gin-gonic/gin"
)

func CommandInjection(c *gin.Context) {
	userInput := c.Query("cmd")
	// Directly passing user input to shell command - Command Injection
	cmd := exec.Command("sh", "-c", userInput)
	output, _ := cmd.Output()
	c.String(200, string(output))
}
