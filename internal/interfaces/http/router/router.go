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

type CapacityApplication interface {
	httphandler.CapacityApplication
	httphandler.CapacityPolicyApplication
}

type Dependencies struct {
	Log                        zerolog.Logger
	StaticFS                   fs.FS
	CapacityApplication        CapacityApplication
	AssetApplication           httphandler.AssetApplication
	NeighborhoodApplication    httphandler.NeighborhoodApplication
	CollectionApplication      CollectionApplication
	CommunityMarketApplication httphandler.CommunityMarketApplication
	DecisionApplication        httphandler.DecisionApplication
	ReviewApplication          httphandler.ReviewApplication
	AccessToken                string
	UserID                     string
	ReadinessChecker           ReadinessChecker
}

var frontendRoutes = map[string]string{
	"/":                                      "index.html",
	"/calculator":                            "calculator.html",
	"/assets":                                "assets.html",
	"/data":                                  "data.html",
	"/watchlist":                             "watchlist.html",
	"/action-window":                         "action-window.html",
	"/neighborhoods":                         "neighborhoods.html",
	"/methods":                               "methods.html",
	"/methods/listings-up-transactions-weak": "methods/listings-up-transactions-weak.html",
	"/methods/asking-price-vs-transactions":  "methods/asking-price-vs-transactions.html",
	"/methods/buyer-window":                  "methods/buyer-window.html",
	"/methods/more-price-cuts":               "methods/more-price-cuts.html",
	"/methods/upgrade-price-gap":             "methods/upgrade-price-gap.html",
	"/methods/monthly-payment-safety":        "methods/monthly-payment-safety.html",
	"/methods/old-home-sale-delay":           "methods/old-home-sale-delay.html",
	"/templates":                             "templates.html",
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
	api.GET("/capacity/assumptions", capacityHandler.GetAssumptions)
	protected.POST("/capacity/calculations", capacityHandler.CreateCalculation)
	protected.GET("/capacity/calculations", capacityHandler.ListCalculations)
	protected.GET("/capacity/calculations/:id", capacityHandler.GetCalculation)

	if deps.AssetApplication != nil {
		assetsHandler := httphandler.NewAssets(deps.AssetApplication, deps.UserID)
		protected.POST("/assets", assetsHandler.Create)
		protected.GET("/assets", assetsHandler.List)
		protected.GET("/assets/:id", assetsHandler.Get)
		protected.PATCH("/assets/:id", assetsHandler.Update)
		protected.DELETE("/assets/:id", assetsHandler.Delete)
	}

	neighborhoodHandler := httphandler.NewNeighborhood(deps.NeighborhoodApplication)
	communityMarketHandler := httphandler.NewCommunityMarket(deps.CommunityMarketApplication)
	watchlistHandler := httphandler.NewWatchlist(deps.NeighborhoodApplication, deps.UserID)
	protected.POST("/neighborhoods", neighborhoodHandler.CreateNeighborhood)
	api.GET("/neighborhoods", neighborhoodHandler.SearchNeighborhoods)
	api.GET("/neighborhoods/:id", neighborhoodHandler.GetNeighborhood)
	api.GET("/neighborhoods/:id/metrics", neighborhoodHandler.GetMetrics)
	api.GET("/neighborhoods/:id/metrics/history", neighborhoodHandler.GetMetricHistory)
	api.GET("/neighborhoods/:id/community-market", communityMarketHandler.GetLatest)
	api.GET("/neighborhoods/:id/community-market/latest", communityMarketHandler.GetLatest)
	api.GET("/neighborhoods/:id/market-listings", communityMarketHandler.ListListings)
	api.GET("/neighborhoods/:id/market-listings/:roomId", communityMarketHandler.GetListing)
	api.GET("/neighborhoods/:id/market-transactions", communityMarketHandler.ListTransactions)
	api.GET("/neighborhoods/:id/market-listings/:roomId/adjustments", communityMarketHandler.ListAdjustments)
	api.GET("/community-market/comparison", communityMarketHandler.Compare)
	protected.POST("/watchlist/items", watchlistHandler.AddItem)
	protected.GET("/watchlist", watchlistHandler.List)

	decisionHandler := httphandler.NewDecision(deps.DecisionApplication)
	protected.GET("/decision/action-window", decisionHandler.GetActionWindow)

	reviewHandler := httphandler.NewReview(deps.ReviewApplication)
	protected.POST("/review-notes", reviewHandler.Create)
	protected.GET("/review-notes", reviewHandler.List)
	protected.GET("/review-notes/:id", reviewHandler.Get)
	protected.PATCH("/review-notes/:id", reviewHandler.Update)

	admin := engine.Group("/admin/api")
	admin.Use(httpmiddleware.AccessAuth(deps.AccessToken))
	dataSourcesHandler := httphandler.NewAdminDataSources(deps.CollectionApplication)
	adminImportsHandler := httphandler.NewAdminImports(deps.CollectionApplication)
	adminCapacityPoliciesHandler := httphandler.NewAdminCapacityPolicies(deps.CapacityApplication)
	admin.POST("/data-sources", dataSourcesHandler.Create)
	admin.GET("/data-sources", dataSourcesHandler.List)
	admin.POST("/imports/json", adminImportsHandler.CreateJSON)
	admin.POST("/imports/csv", adminImportsHandler.CreateCSV)
	admin.GET("/imports/csv/template", adminImportsHandler.GetCSVTemplate)
	admin.GET("/imports", adminImportsHandler.List)
	admin.GET("/imports/:id", adminImportsHandler.GetDetail)
	admin.GET("/capacity/policies", adminCapacityPoliciesHandler.List)
	admin.POST("/capacity/policies", adminCapacityPoliciesHandler.Create)
	admin.POST("/community-market/imports/csv", communityMarketHandler.ImportCSV)
	admin.POST("/community-market/imports/fangjian", communityMarketHandler.ImportFangjian)

	fileServer := http.FileServer(http.FS(staticFS))
	engine.GET("/_next/*filepath", gin.WrapH(fileServer))
	engine.GET("/icon.svg", gin.WrapH(fileServer))
	engine.GET("/404.html", gin.WrapH(fileServer))

	for route, name := range frontendRoutes {
		engine.GET(route, serveFrontendFile(staticFS, name))
		rscRoute := route + ".txt"
		if route == "/" {
			rscRoute = "/index.txt"
		}
		engine.GET(rscRoute, serveFrontendRSCFile(staticFS, strings.TrimSuffix(name, ".html")+".txt"))
	}
	engine.GET("/data/imports/:id", serveFrontendImport(staticFS))

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
	missing := make([]string, 0, 6)
	if deps.CapacityApplication == nil {
		missing = append(missing, "CapacityApplication")
	}
	if deps.NeighborhoodApplication == nil {
		missing = append(missing, "NeighborhoodApplication")
	}
	if deps.CollectionApplication == nil {
		missing = append(missing, "CollectionApplication")
	}
	if deps.CommunityMarketApplication == nil {
		missing = append(missing, "CommunityMarketApplication")
	}
	if deps.DecisionApplication == nil {
		missing = append(missing, "DecisionApplication")
	}
	if deps.ReviewApplication == nil {
		missing = append(missing, "ReviewApplication")
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

func serveFrontendRSCFile(staticFS fs.FS, name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := fs.ReadFile(staticFS, name)
		if err != nil {
			jsonNotFound(c)
			return
		}
		c.Data(http.StatusOK, "text/x-component; charset=utf-8", body)
	}
}

func serveFrontendImport(staticFS fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Param("id"), ".txt") {
			serveFrontendRSCFile(staticFS, "data/imports/_.txt")(c)
			return
		}
		serveFrontendFile(staticFS, "data/imports/_.html")(c)
	}
}
