package router

import (
	"context"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"billing-app/backend/internal/handlers"
	"billing-app/backend/internal/middleware"
	"billing-app/backend/internal/service"
)

const readinessTimeout = 2 * time.Second

type Pinger interface {
	PingContext(ctx context.Context) error
}

type Dependencies struct {
	Logger       *slog.Logger
	DB           Pinger
	Customers    *service.CustomerService
	Products     *service.ProductService
	Invoices     *service.InvoiceService
	InvoiceItems *service.InvoiceItemService
}

var registerTagNamesOnce sync.Once

func New(d Dependencies) *gin.Engine {
	registerTagNamesOnce.Do(useJSONFieldNamesInValidationErrors)

	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.HandleMethodNotAllowed = true

	r.Use(middleware.RequestLogger(d.Logger), middleware.Recovery(d.Logger))

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "route not found"}})
	})
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": gin.H{"code": "method_not_allowed", "message": "method not allowed"}})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ready", readiness(d.DB, d.Logger))

	customers := handlers.NewCustomerHandler(d.Customers, d.Logger)
	products := handlers.NewProductHandler(d.Products, d.Logger)
	invoices := handlers.NewInvoiceHandler(d.Invoices, d.Logger)
	items := handlers.NewInvoiceItemHandler(d.InvoiceItems, d.Logger)

	v1 := r.Group("/api/v1", middleware.Auth())

	v1.GET("/customers", customers.List)
	v1.POST("/customers", customers.Create)
	v1.GET("/customers/:id", customers.Get)
	v1.PUT("/customers/:id", customers.Update)
	v1.DELETE("/customers/:id", customers.Delete)

	v1.GET("/products", products.List)
	v1.POST("/products", products.Create)
	v1.GET("/products/:id", products.Get)
	v1.PUT("/products/:id", products.Update)
	v1.DELETE("/products/:id", products.Delete)

	v1.GET("/invoices", invoices.List)
	v1.POST("/invoices", invoices.Create)
	v1.GET("/invoices/:id", invoices.Get)
	v1.PUT("/invoices/:id", invoices.Update)
	v1.DELETE("/invoices/:id", invoices.Delete)

	v1.GET("/invoices/:id/items", items.List)
	v1.POST("/invoices/:id/items", items.Create)
	v1.PUT("/invoices/:id/items/:itemId", items.Update)
	v1.DELETE("/invoices/:id/items/:itemId", items.Delete)

	return r
}

func readiness(db Pinger, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			log.WarnContext(ctx, "readiness check failed", slog.Any("error", err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}

func useJSONFieldNamesInValidationErrors() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			return ""
		}
		return name
	})
}
