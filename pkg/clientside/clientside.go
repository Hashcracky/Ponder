// Package clientside is a package for the client side of the application
package clientside

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// HTML /
// SCOPE: PUBLIC

// ClientIndexHandler is a handler for the index page of the client side
func ClientIndexHandler(c *gin.Context) {
	if _, err := os.Stat("/etc/ponder/static/index.html"); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "404 page not found")
		return
	} else if err != nil {
		c.String(http.StatusInternalServerError, "500 internal server error")
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{})
}
