package controller

import (
	"net/http"

	"goempty/internal/controller/response"
	"goempty/internal/service"

	"github.com/gin-gonic/gin"
)

// HelloController handles greeting requests.
type HelloController struct {
	helloService *service.HelloService
}

// NewHelloController creates a controller with its service dependency.
func NewHelloController(helloService *service.HelloService) *HelloController {
	return &HelloController{helloService: helloService}
}

// Hello returns a greeting as JSON.
func (h *HelloController) Hello(c *gin.Context) {
	response.Success(c, http.StatusOK, gin.H{"message": h.helloService.Hello()})
}
