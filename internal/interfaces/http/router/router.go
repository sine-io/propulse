package router

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	webembed "github.com/sine-io/propulse/apps/web/embed"
	httphandler "github.com/sine-io/propulse/internal/interfaces/http/handler"
	httpmiddleware "github.com/sine-io/propulse/internal/interfaces/http/middleware"
)

type ReadinessChecker interface {
	Check(context.Context) error
}

type CollectionApplication interface {
	httphandler.CollectionApplication
	httphandler.DataSourceApplication
}

type Dependencies struct {
	Log                     zerolog.Logger
	StaticFS                fs.FS
	CapacityApplication     httphandler.CapacityApplication
	NeighborhoodApplication httphandler.NeighborhoodApplication
	CollectionApplication   CollectionApplication
	DecisionApplication     httphandler.DecisionApplication
	AccessToken             string
	UserID                  string
	ReadinessChecker        ReadinessChecker
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

func New(deps Dependencies) (*gin.Engine, error) {
	if err := validateDependencies(deps); err != nil {
		return nil, err
	}

	staticFS := deps.StaticFS
	if staticFS == nil {
		staticFS = webembed.Embedded()
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
		if deps.ReadinessChecker == nil {
			serviceUnavailable(c)
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := deps.ReadinessChecker.Check(ctx); err != nil {
			serviceUnavailable(c)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	api := engine.Group("/api/v1")
	protected := api.Group("")
	protected.Use(httpmiddleware.AccessAuth(deps.AccessToken))
	protected.GET("/access", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "unlocked"})
	})

	capacityHandler := httphandler.NewCapacity(deps.CapacityApplication, deps.UserID)
	protected.POST("/capacity/calculations", capacityHandler.CreateCalculation)
	protected.GET("/capacity/calculations/:id", capacityHandler.GetCalculation)

	neighborhoodHandler := httphandler.NewNeighborhood(deps.NeighborhoodApplication)
	watchlistHandler := httphandler.NewWatchlist(deps.NeighborhoodApplication, deps.UserID)
	protected.POST("/neighborhoods", neighborhoodHandler.CreateNeighborhood)
	api.GET("/neighborhoods/:id", neighborhoodHandler.GetNeighborhood)
	api.GET("/neighborhoods/:id/metrics", neighborhoodHandler.GetMetrics)
	protected.POST("/watchlist/items", watchlistHandler.AddItem)
	protected.GET("/watchlist", watchlistHandler.List)

	decisionHandler := httphandler.NewDecision(deps.DecisionApplication)
	protected.GET("/decision/action-window", decisionHandler.GetActionWindow)

	admin := engine.Group("/admin/api")
	admin.Use(httpmiddleware.AccessAuth(deps.AccessToken))
	dataSourcesHandler := httphandler.NewAdminDataSources(deps.CollectionApplication)
	adminImportsHandler := httphandler.NewAdminImports(deps.CollectionApplication)
	admin.POST("/data-sources", dataSourcesHandler.Create)
	admin.GET("/data-sources", dataSourcesHandler.List)
	admin.POST("/imports/json", adminImportsHandler.CreateJSON)
	admin.GET("/imports/:id", adminImportsHandler.GetDetail)

	fileServer := http.FileServer(http.FS(staticFS))
	engine.GET("/_next/*filepath", gin.WrapH(fileServer))
	engine.GET("/icon.svg", gin.WrapH(fileServer))
	engine.GET("/404.html", gin.WrapH(fileServer))

	for route, name := range frontendRoutes {
		engine.GET(route, serveFrontendFile(staticFS, name))
	}

	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1") || strings.HasPrefix(c.Request.URL.Path, "/admin/api") {
			jsonNotFound(c)
			return
		}
		http.NotFound(c.Writer, c.Request)
	})

	return engine, nil
}

func validateDependencies(deps Dependencies) error {
	missing := make([]string, 0, 4)
	if deps.CapacityApplication == nil {
		missing = append(missing, "CapacityApplication")
	}
	if deps.NeighborhoodApplication == nil {
		missing = append(missing, "NeighborhoodApplication")
	}
	if deps.CollectionApplication == nil {
		missing = append(missing, "CollectionApplication")
	}
	if deps.DecisionApplication == nil {
		missing = append(missing, "DecisionApplication")
	}
	if len(missing) > 0 {
		return fmt.Errorf("router dependencies are required: %s", strings.Join(missing, ", "))
	}
	return nil
}

func serviceUnavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"code":    "not_ready",
			"message": "service dependencies are not ready",
		},
	})
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
