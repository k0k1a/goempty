package app

import (
	"net/http"

	"goempty/internal/controller"
	"goempty/internal/controller/middleware"
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
	engine.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/readyz", func(c *gin.Context) { c.Status(http.StatusOK) })

	hello := controller.NewHelloController(deps.Hello)
	engine.GET("/hello", hello.Hello)
	return engine
}
