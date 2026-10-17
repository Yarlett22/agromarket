// Catalog Service: catálogo de productos agroexportables.
// - Cache del catálogo en Redis (patrón cache-aside).
// - Consume StockUpdated para mantener su propia copia del stock e invalidar la cache.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"agromarket/internal/common"

	"github.com/redis/go-redis/v9"
)

type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Variety     string  `json:"variety"`
	Origin      string  `json:"origin"`
	Unit        string  `json:"unit"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
	Image       string  `json:"image"`
	Description string  `json:"description"`
}

const cacheKey = "catalog:products"

var (
	db  *sql.DB
	rdb *redis.Client
)

func loadProducts() ([]Product, error) {
	rows, err := db.Query(`SELECT id, name, variety, origin, unit, price, stock, image, description FROM catalog.products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Variety, &p.Origin, &p.Unit, &p.Price, &p.Stock, &p.Image, &p.Description); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func listProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if cached, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cached))
		return
	}
	products, err := loadProducts()
	if err != nil {
		common.Error(w, 500, err.Error())
		return
	}
	data, _ := json.Marshal(products)
	rdb.Set(ctx, cacheKey, data, 60*time.Second)
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		common.Error(w, 400, "id inválido")
		return
	}
	var p Product
	err = db.QueryRow(`SELECT id, name, variety, origin, unit, price, stock, image, description FROM catalog.products WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &p.Variety, &p.Origin, &p.Unit, &p.Price, &p.Stock, &p.Image, &p.Description)
	if err != nil {
		common.Error(w, 404, "producto no encontrado")
		return
	}
	common.JSON(w, 200, p)
}

func main() {
	db = common.MustPostgres()
	rdb = common.MustRedis()
	nc := common.MustNATS("catalog-service")

	// Event consumer: sincroniza stock y limpia la cache.
	common.Subscribe(nc, common.SubjStockUpdated, "catalog-service", func(e common.StockUpdated) {
		for _, c := range e.Changes {
			if _, err := db.Exec(`UPDATE catalog.products SET stock=$1 WHERE id=$2`, c.Stock, c.ProductID); err != nil {
				log.Printf("✖ error actualizando stock: %v", err)
			}
		}
		rdb.Del(context.Background(), cacheKey)
		log.Printf("catálogo sincronizado tras pedido #%d (cache invalidada)", e.OrderID)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /products", listProducts)
	mux.HandleFunc("GET /products/{id}", getProduct)
	common.Serve("catalog-service", common.Env("PORT", "8002"), mux)
}
