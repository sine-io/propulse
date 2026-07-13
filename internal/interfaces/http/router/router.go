package router

import (
	"io/fs"
	"net/http"
	"strings"

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

type Dependencies struct {
	Log                     zerolog.Logger
	StaticFS                fs.FS
	CapacityApplication     httphandler.CapacityApplication
	NeighborhoodApplication httphandler.NeighborhoodApplication
	CollectionApplication   httphandler.CollectionApplication
	DecisionApplication     httphandler.DecisionApplication
	AdminAPIToken           string
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
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	api := engine.Group("/api/v1")
	capacityApp := deps.CapacityApplication
	if capacityApp == nil {
		capacityApp = appcapacity.NewService(newInMemoryCalculationRepository(), nil, nil)
	}
	capacityHandler := httphandler.NewCapacity(capacityApp)
	api.POST("/capacity/calculations", capacityHandler.CreateCalculation)
	api.GET("/capacity/calculations/:id", capacityHandler.GetCalculation)

	neighborhoodApp := deps.NeighborhoodApplication
	if neighborhoodApp == nil {
		neighborhoodApp = appneighborhood.NewService(newInMemoryNeighborhoodRepository())
	}
	neighborhoodHandler := httphandler.NewNeighborhood(neighborhoodApp)
	watchlistHandler := httphandler.NewWatchlist(neighborhoodApp)
	api.POST("/neighborhoods", neighborhoodHandler.CreateNeighborhood)
	api.GET("/neighborhoods/:id", neighborhoodHandler.GetNeighborhood)
	api.GET("/neighborhoods/:id/metrics", neighborhoodHandler.GetMetrics)
	api.POST("/watchlist/items", watchlistHandler.AddItem)
	api.GET("/watchlist", watchlistHandler.List)

	decisionApp := deps.DecisionApplication
	if decisionApp == nil {
		decisionApp = appdecision.NewService(capacityApp, neighborhoodApp)
	}
	decisionHandler := httphandler.NewDecision(decisionApp)
	api.GET("/decision/action-window", decisionHandler.GetActionWindow)

	admin := engine.Group("/admin/api")
	admin.Use(httpmiddleware.AdminAuth(deps.AdminAPIToken))
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
