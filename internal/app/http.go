package app

import (
	"net/http"

	"goempty/internal/controller"
	"goempty/internal/controller/middleware"
	"goempty/internal/controller/response"
	"goempty/internal/service"
	"goempty/pkg/httpserver"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// httpDependencies contains the services used to assemble HTTP controllers.
type httpDependencies struct {
	Hello *service.HelloService
}

// newHTTPServer assembles the HTTP transport with the application's Gin handler.
func newHTTPServer(cfg httpserver.Config, deps *httpDependencies) (*httpserver.Server, error) {
	if err := response.InitBindingValidator(); err != nil {
		return nil, err
	}
	return httpserver.New(cfg, newHTTPHandler(deps))
}

// newHTTPHandler assembles middleware and business routes before serving starts.
func newHTTPHandler(deps *httpDependencies) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.RequestID())
	engine.Use(middleware.AccessLog())
	engine.Use(middleware.Recovery())
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "Authorization", "X-Request-ID")
	corsConfig.ExposeHeaders = []string{"X-Request-ID"}
	engine.Use(cors.New(corsConfig))
	engine.HandleMethodNotAllowed = true
	engine.NoRoute(controller.RouteNotFound)
	engine.NoMethod(controller.MethodNotAllowed)
	engine.GET("/healthz", healthz)
	engine.GET("/readyz", readyz)

	hello := controller.NewHelloController(deps.Hello)
	engine.GET("/hello", hello.Hello)
	return engine
}

// healthz reports that the HTTP process is alive.
// @Summary Check liveness
// @Tags health
// @Success 200 "OK; empty response body"
// @Header 200 {string} X-Request-ID "Request ID"
// @Router /healthz [get]
func healthz(c *gin.Context) {
	c.Status(http.StatusOK)
}

// readyz preserves the existing readiness response without checking dependencies.
// @Summary Check readiness
// @Description Returns 200 with an empty body; does not check MySQL or Redis connectivity.
// @Tags health
// @Success 200 "OK; empty response body"
// @Header 200 {string} X-Request-ID "Request ID"
// @Router /readyz [get]
func readyz(c *gin.Context) {
	c.Status(http.StatusOK)
}
