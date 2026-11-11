package controller

import (
	"goempty/internal/controller/response"
	"goempty/internal/entity"

	"github.com/gin-gonic/gin"
)

// RouteNotFound handles requests that do not match a registered route.
func RouteNotFound(c *gin.Context) {
	response.Failure(c, entity.ErrNotFound.Code, "接口不存在")
}

// MethodNotAllowed handles unsupported methods on registered routes.
func MethodNotAllowed(c *gin.Context) {
	response.Failure(c, entity.ErrInvalidArgument.Code, "请求方法不支持")
}
