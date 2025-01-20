package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var isAlive = true
var isReady = false

func HandleHealthCheck(c *gin.Context) {
	c.Status(http.StatusOK)
}

func HandleLivenessCheck(c *gin.Context) {
	if isAlive {
		c.Status(http.StatusOK)
	} else {
		c.Status(http.StatusInternalServerError)
	}
}

func HandleReadinessCheck(c *gin.Context) {
	if isReady {
		c.Status(http.StatusOK)
	} else {
		c.Status(http.StatusServiceUnavailable)
	}
}

func SetIsReadyGlobally() {
	isReady = true
}

func SetLivenessGlobally(val bool) {
	isAlive = val
}
