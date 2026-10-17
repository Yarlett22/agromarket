// Analytics Service (event consumer): agrega métricas de negocio en Redis a partir
// de los eventos (pedidos, ingresos, rechazos, unidades vendidas por producto).
// Expone GET /analytics.
package main

import (
	"context"
	"net/http"
	"strconv"

	"agromarket/internal/common"
)

const key = "analytics"

func main() {
	rdb := common.MustRedis()
	nc := common.MustNATS("analytics-service")
	ctx := context.Background()

	common.Subscribe(nc, common.SubjOrderCreated, "analytics-service", func(e common.OrderCreated) {
		rdb.HIncrBy(ctx, key, "orders_created", 1)
		rdb.HIncrBy(ctx, key, "events_total", 1)
	})
	common.Subscribe(nc, common.SubjPaymentCompleted, "analytics-service", func(e common.PaymentResult) {
		rdb.HIncrBy(ctx, key, "payments_completed", 1)
		rdb.HIncrByFloat(ctx, key, "revenue", e.Total)
		rdb.HIncrBy(ctx, key, "events_total", 1)
		for _, it := range e.Items {
			rdb.ZIncrBy(ctx, key+":units", float64(it.Quantity), it.Name)
		}
	})
	common.Subscribe(nc, common.SubjPaymentFailed, "analytics-service", func(e common.PaymentResult) {
		rdb.HIncrBy(ctx, key, "payments_failed", 1)
		rdb.HIncrBy(ctx, key, "events_total", 1)
	})
	common.Subscribe(nc, common.SubjStockUpdated, "analytics-service", func(e common.StockUpdated) {
		rdb.HIncrBy(ctx, key, "stock_updates", 1)
		rdb.HIncrBy(ctx, key, "events_total", 1)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /analytics", func(w http.ResponseWriter, r *http.Request) {
		h, _ := rdb.HGetAll(r.Context(), key).Result()
		num := func(k string) float64 { f, _ := strconv.ParseFloat(h[k], 64); return f }
		top, _ := rdb.ZRevRangeWithScores(r.Context(), key+":units", 0, 4).Result()
		topOut := []map[string]any{}
		for _, z := range top {
			topOut = append(topOut, map[string]any{"name": z.Member, "units": z.Score})
		}
		common.JSON(w, 200, map[string]any{
			"orders_created":     num("orders_created"),
			"payments_completed": num("payments_completed"),
			"payments_failed":    num("payments_failed"),
			"stock_updates":      num("stock_updates"),
			"events_total":       num("events_total"),
			"revenue":            num("revenue"),
			"top_products":       topOut,
		})
	})
	common.Serve("analytics-service", common.Env("PORT", "8005"), mux)
}
