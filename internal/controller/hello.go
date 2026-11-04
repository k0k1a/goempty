package controller

import (
	"net/http"

	"goempty/internal/controller/response"
	"goempty/internal/entity"
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
// @Summary Get a greeting
// @Tags hello
// @Produce json
// @Success 200 {object} response.Response[entity.HelloResponse] "Greeting returned successfully; code is 0"
// @Failure 500 {object} response.Response[any] "Internal error; code is -1, data is null"
// @Header all {string} X-Request-ID "Request ID"
// @Router /hello [get]
func (h *HelloController) Hello(c *gin.Context) {
	response.Success(c, http.StatusOK, entity.HelloResponse{Message: h.helloService.Hello()})
}
