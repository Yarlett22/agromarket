// Notification Service (event consumer): escucha todos los eventos de negocio y
// genera notificaciones por usuario, guardadas en Redis (lista por usuario).
// Expone GET /notifications para que el frontend las muestre.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"agromarket/internal/common"

	"github.com/redis/go-redis/v9"
)

type Notification struct {
	Title   string    `json:"title"`
	Message string    `json:"message"`
	Kind    string    `json:"kind"` // info | success | error
	Event   string    `json:"event"`
	OrderID int       `json:"order_id"`
	At      time.Time `json:"at"`
}

var rdb *redis.Client

func push(userID int, n Notification) {
	n.At = time.Now()
	data, _ := json.Marshal(n)
	key := fmt.Sprintf("notifications:%d", userID)
	ctx := context.Background()
	rdb.LPush(ctx, key, data)
	rdb.LTrim(ctx, key, 0, 29)
}

func list(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.Atoi(r.Header.Get("X-User-Id"))
	if err != nil {
		common.Error(w, 401, "no autenticado")
		return
	}
	raw, _ := rdb.LRange(r.Context(), fmt.Sprintf("notifications:%d", uid), 0, 29).Result()
	out := []json.RawMessage{}
	for _, s := range raw {
		out = append(out, json.RawMessage(s))
	}
	common.JSON(w, 200, out)
}

func main() {
	rdb = common.MustRedis()
	nc := common.MustNATS("notification-service")

	common.Subscribe(nc, common.SubjOrderCreated, "notification-service", func(e common.OrderCreated) {
		push(e.UserID, Notification{Title: "Pedido recibido", Kind: "info", Event: "OrderCreated", OrderID: e.OrderID,
			Message: fmt.Sprintf("Tu pedido #%d por S/ %.2f fue registrado.", e.OrderID, e.Total)})
	})
	common.Subscribe(nc, common.SubjPaymentCompleted, "notification-service", func(e common.PaymentResult) {
		push(e.UserID, Notification{Title: "Pago aprobado", Kind: "success", Event: "PaymentCompleted", OrderID: e.OrderID,
			Message: fmt.Sprintf("Pago %s confirmado para el pedido #%d.", e.PaymentID, e.OrderID)})
	})
	common.Subscribe(nc, common.SubjPaymentFailed, "notification-service", func(e common.PaymentResult) {
		push(e.UserID, Notification{Title: "Pago rechazado", Kind: "error", Event: "PaymentFailed", OrderID: e.OrderID,
			Message: fmt.Sprintf("Pedido #%d: %s", e.OrderID, e.Reason)})
	})
	common.Subscribe(nc, common.SubjStockUpdated, "notification-service", func(e common.StockUpdated) {
		push(e.UserID, Notification{Title: "Pedido confirmado", Kind: "success", Event: "StockUpdated", OrderID: e.OrderID,
			Message: fmt.Sprintf("El stock fue reservado. Tu pedido #%d está listo para despacho.", e.OrderID)})
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /notifications", list)
	common.Serve("notification-service", common.Env("PORT", "8004"), mux)
}
