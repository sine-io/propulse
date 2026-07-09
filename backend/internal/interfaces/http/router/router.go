package router

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	httpmiddleware "github.com/propulse/propulse/backend/internal/interfaces/http/middleware"
	"github.com/propulse/propulse/backend/web"
	"github.com/rs/zerolog"
)

type Dependencies struct {
	Log      zerolog.Logger
	StaticFS fs.FS
}

var frontendRoutes = map[string]string{
	"/":              "index.html",
	"/calculator":    "calculator.html",
	"/watchlist":     "watchlist.html",
	"/action-window": "action-window.html",
	"/neighborhoods": "neighborhoods.html",
	"/methods":       "methods.html",
	"/templates":     "templates.html",
}

func New(deps Dependencies) *gin.Engine {
	staticFS := deps.StaticFS
	if staticFS == nil {
		staticFS = web.Embedded()
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(httpmiddleware.RequestID())
	engine.Use(httpmiddleware.Logging(deps.Log))

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	api := engine.Group("/api/v1")
	api.Any("", jsonNotFound)
	api.Any("/*path", jsonNotFound)

	admin := engine.Group("/admin/api")
	admin.Any("", jsonNotFound)
	admin.Any("/*path", jsonNotFound)

	fileServer := http.FileServer(http.FS(staticFS))
	engine.GET("/_next/*filepath", gin.WrapH(fileServer))
	engine.GET("/icon.svg", gin.WrapH(fileServer))
	engine.GET("/404.html", gin.WrapH(fileServer))

	for route, name := range frontendRoutes {
		engine.GET(route, serveFrontendFile(staticFS, name))
	}

	return engine
}

func jsonNotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
}

func serveFrontendFile(staticFS fs.FS, name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := fs.ReadFile(staticFS, name)
		if err != nil {
			jsonNotFound(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", body)
	}
}
