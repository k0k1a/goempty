package controller

import (
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
// @Failure 400 {object} response.Response[any] "Internal error; code is -1, data is null"
// @Header all {string} X-Request-ID "Request ID"
// @Router /hello [get]
func (h *HelloController) Hello(c *gin.Context) {
	response.Success(c, entity.HelloResponse{Message: h.helloService.Hello()})
}

// PublishMessage publishes one hello greeting message to Kafka.
// @Summary Publish a hello message
// @Tags hello
// @Accept json
// @Produce json
// @Param request body entity.HelloGreetingMessage true "Greeting message"
// @Success 200 {object} response.Response[entity.HelloResponse] "Message published successfully; code is 0"
// @Failure 400 {object} response.Response[any] "Invalid request or publish failure"
// @Header all {string} X-Request-ID "Request ID"
// @Router /hello/message [post]
func (h *HelloController) PublishMessage(c *gin.Context) {
	var message entity.HelloGreetingMessage
	if err := c.ShouldBindJSON(&message); err != nil {
		response.RespondError(c, entity.ErrInvalidArgument)
		return
	}
	if err := h.helloService.PublishGreetingMessage(c.Request.Context(), message); err != nil {
		response.RespondError(c, err)
		return
	}
	response.Success(c, entity.HelloResponse{Message: message.Message})
}
