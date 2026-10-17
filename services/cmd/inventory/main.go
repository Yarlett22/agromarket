// Inventory Service (event consumer): consume PaymentCompleted, descuenta stock
// en su propia base (inventory.stock) dentro de una transacción y publica StockUpdated.
package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"agromarket/internal/common"

	"github.com/nats-io/nats.go"
)

func main() {
	db := common.MustPostgres()
	nc := common.MustNATS("inventory-service")

	delay, _ := time.ParseDuration(common.Env("INVENTORY_DELAY", "1s"))

	common.Subscribe(nc, common.SubjPaymentCompleted, "inventory-service", func(e common.PaymentResult) {
		go func() {
			time.Sleep(delay) // simula picking en almacén
			reserve(db, nc, e)
		}()
	})

	common.Serve("inventory-service", common.Env("PORT", "8007"), http.NewServeMux())
}

func reserve(db *sql.DB, nc *nats.Conn, e common.PaymentResult) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("✖ %v", err)
		return
	}
	defer tx.Rollback()

	var changes []common.StockLevel
	for _, it := range e.Items {
		var stock int
		err := tx.QueryRow(`UPDATE inventory.stock SET quantity = GREATEST(quantity - $1, 0), updated_at = now()
			WHERE product_id = $2 RETURNING quantity`, it.Quantity, it.ProductID).Scan(&stock)
		if err != nil {
			log.Printf("✖ error descontando stock del producto %d: %v", it.ProductID, err)
			return
		}
		changes = append(changes, common.StockLevel{ProductID: it.ProductID, Stock: stock})
		log.Printf("📦 producto %d: -%d → stock %d", it.ProductID, it.Quantity, stock)
	}
	if err := tx.Commit(); err != nil {
		log.Printf("✖ %v", err)
		return
	}
	common.Publish(nc, common.SubjStockUpdated, common.StockUpdated{OrderID: e.OrderID, UserID: e.UserID, Changes: changes})
}
