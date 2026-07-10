package router

import (
	"context"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	webembed "github.com/sine-io/propulse/apps/web/embed"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	appdecision "github.com/sine-io/propulse/internal/application/decision"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	httphandler "github.com/sine-io/propulse/internal/interfaces/http/handler"
	httpmiddleware "github.com/sine-io/propulse/internal/interfaces/http/middleware"
)

type ReadinessChecker interface {
	Check(context.Context) error
}

type Dependencies struct {
	Log                     zerolog.Logger
	StaticFS                fs.FS
	CapacityApplication     httphandler.CapacityApplication
	NeighborhoodApplication httphandler.NeighborhoodApplication
	CollectionApplication   httphandler.CollectionApplication
	DecisionApplication     httphandler.DecisionApplication
	AccessToken             string
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

func New(deps Dependencies) *gin.Engine {
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
	capacityApp := deps.CapacityApplication
	if capacityApp == nil {
		capacityApp = appcapacity.NewService(newInMemoryCalculationRepository(), nil, nil)
	}
	capacityHandler := httphandler.NewCapacity(capacityApp)
	protected.POST("/capacity/calculations", capacityHandler.CreateCalculation)
	protected.GET("/capacity/calculations/:id", capacityHandler.GetCalculation)

	neighborhoodApp := deps.NeighborhoodApplication
	if neighborhoodApp == nil {
		neighborhoodApp = appneighborhood.NewService(newInMemoryNeighborhoodRepository())
	}
	neighborhoodHandler := httphandler.NewNeighborhood(neighborhoodApp)
	watchlistHandler := httphandler.NewWatchlist(neighborhoodApp)
	protected.POST("/neighborhoods", neighborhoodHandler.CreateNeighborhood)
	api.GET("/neighborhoods/:id", neighborhoodHandler.GetNeighborhood)
	api.GET("/neighborhoods/:id/metrics", neighborhoodHandler.GetMetrics)
	protected.POST("/watchlist/items", watchlistHandler.AddItem)
	protected.GET("/watchlist", watchlistHandler.List)

	decisionApp := deps.DecisionApplication
	if decisionApp == nil {
		decisionApp = appdecision.NewService(capacityApp, neighborhoodApp)
	}
	decisionHandler := httphandler.NewDecision(decisionApp)
	protected.GET("/decision/action-window", decisionHandler.GetActionWindow)

	admin := engine.Group("/admin/api")
	admin.Use(httpmiddleware.AccessAuth(deps.AccessToken))
	collectionApp := deps.CollectionApplication
	if collectionApp == nil {
		collectionApp = appcollection.NewService(newInMemoryCollectionRepository(), nil, nil)
	}
	adminImportsHandler := httphandler.NewAdminImports(collectionApp)
	admin.POST("/imports", adminImportsHandler.CreateImport)

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

	return engine
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
