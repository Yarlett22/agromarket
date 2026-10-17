// Payment Service (event consumer): consume OrderCreated, simula la pasarela de pago
// y publica PaymentCompleted o PaymentFailed.
// Regla de negocio de la simulación: pedidos mayores al límite de crédito se rechazan.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"agromarket/internal/common"

	"github.com/nats-io/nats.go"
)

func main() {
	db := common.MustPostgres()
	nc := common.MustNATS("payment-service")

	delay, _ := time.ParseDuration(common.Env("PAYMENT_DELAY", "2s"))
	limit, _ := strconv.ParseFloat(common.Env("CREDIT_LIMIT", "5000"), 64)

	common.Subscribe(nc, common.SubjOrderCreated, "payment-service", func(e common.OrderCreated) {
		go process(db, nc, e, delay, limit)
	})

	common.Serve("payment-service", common.Env("PORT", "8006"), http.NewServeMux())
}

func process(db *sql.DB, nc *nats.Conn, e common.OrderCreated, delay time.Duration, limit float64) {
	log.Printf("💳 procesando pago del pedido #%d por S/ %.2f ...", e.OrderID, e.Total)
	time.Sleep(delay) // simula latencia de la pasarela

	res := common.PaymentResult{OrderID: e.OrderID, UserID: e.UserID, Items: e.Items, Total: e.Total}
	status := "APROBADO"
	if e.Total > limit {
		status = "RECHAZADO"
		res.Reason = fmt.Sprintf("El monto excede el límite de crédito (S/ %.0f)", limit)
	} else {
		res.PaymentID = fmt.Sprintf("PAY-%d-%d", e.OrderID, time.Now().Unix())
	}

	if _, err := db.Exec(`INSERT INTO payments.payments (order_id, user_id, amount, status, reference, reason)
		VALUES ($1,$2,$3,$4,$5,$6)`, e.OrderID, e.UserID, e.Total, status, res.PaymentID, res.Reason); err != nil {
		log.Printf("✖ error guardando pago: %v", err)
	}

	if status == "APROBADO" {
		common.Publish(nc, common.SubjPaymentCompleted, res)
	} else {
		common.Publish(nc, common.SubjPaymentFailed, res)
	}
}
