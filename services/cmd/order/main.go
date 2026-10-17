// Order Service: crea pedidos (síncrono) y publica OrderCreated en el Event Bus.
// Además consume PaymentCompleted / PaymentFailed / StockUpdated para avanzar el
// estado del pedido: PENDIENTE → PAGADO → CONFIRMADO (o RECHAZADO).
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"agromarket/internal/common"

	"github.com/nats-io/nats.go"
)

var (
	db         *sql.DB
	nc         *nats.Conn
	catalogURL = common.Env("CATALOG_URL", "http://localhost:8002")
	httpClient = &http.Client{Timeout: 3 * time.Second}
)

type product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

type orderRequest struct {
	Items []struct {
		ProductID int `json:"product_id"`
		Quantity  int `json:"quantity"`
	} `json:"items"`
}

type Order struct {
	ID        int                `json:"id"`
	UserID    int                `json:"user_id"`
	Status    string             `json:"status"`
	Total     float64            `json:"total"`
	Items     []common.OrderItem `json:"items"`
	Timeline  json.RawMessage    `json:"timeline"`
	Reason    string             `json:"reason,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

// fetchProduct consulta de forma síncrona (REST) al Catalog Service.
func fetchProduct(id int) (*product, error) {
	resp, err := httpClient.Get(fmt.Sprintf("%s/products/%d", catalogURL, id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("producto %d no existe", id)
	}
	var p product
	return &p, json.NewDecoder(resp.Body).Decode(&p)
}

func userID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.Header.Get("X-User-Id")) // inyectada por el API Gateway
	return id, err == nil && id > 0
}

func createOrder(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		common.Error(w, 401, "no autenticado")
		return
	}
	var req orderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Items) == 0 {
		common.Error(w, 400, "el pedido debe tener al menos un producto")
		return
	}
	var items []common.OrderItem
	total := 0.0
	for _, it := range req.Items {
		if it.Quantity <= 0 {
			common.Error(w, 400, "cantidad inválida")
			return
		}
		p, err := fetchProduct(it.ProductID)
		if err != nil {
			common.Error(w, 400, err.Error())
			return
		}
		if p.Stock < it.Quantity {
			common.Error(w, 409, fmt.Sprintf("stock insuficiente de %s (disponible: %d)", p.Name, p.Stock))
			return
		}
		items = append(items, common.OrderItem{ProductID: p.ID, Name: p.Name, Quantity: it.Quantity, UnitPrice: p.Price})
		total += p.Price * float64(it.Quantity)
	}
	itemsJSON, _ := json.Marshal(items)
	var o Order
	err := db.QueryRow(`
		INSERT INTO orders.orders (user_id, status, total, items, timeline)
		VALUES ($1, 'PENDIENTE', $2, $3, jsonb_build_array(jsonb_build_object('status','PENDIENTE','event','OrderCreated','at',now())))
		RETURNING id, created_at`, uid, total, itemsJSON).Scan(&o.ID, &o.CreatedAt)
	if err != nil {
		common.Error(w, 500, err.Error())
		return
	}
	// Publicación asíncrona: el Order Service no conoce a sus consumidores.
	common.Publish(nc, common.SubjOrderCreated, common.OrderCreated{
		OrderID: o.ID, UserID: uid, Items: items, Total: total, CreatedAt: o.CreatedAt,
	})
	common.JSON(w, 202, map[string]any{"id": o.ID, "status": "PENDIENTE", "total": total,
		"message": "Pedido recibido. Se procesará de forma asíncrona."})
}

const orderCols = `id, user_id, status, total, items, timeline, coalesce(reason,''), created_at`

func scanOrder(s interface{ Scan(...any) error }) (Order, error) {
	var o Order
	var items []byte
	err := s.Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &items, &o.Timeline, &o.Reason, &o.CreatedAt)
	_ = json.Unmarshal(items, &o.Items)
	return o, err
}

func listOrders(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		common.Error(w, 401, "no autenticado")
		return
	}
	rows, err := db.Query(`SELECT `+orderCols+` FROM orders.orders WHERE user_id=$1 ORDER BY id DESC LIMIT 50`, uid)
	if err != nil {
		common.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		if o, err := scanOrder(rows); err == nil {
			out = append(out, o)
		}
	}
	common.JSON(w, 200, out)
}

func getOrder(w http.ResponseWriter, r *http.Request) {
	uid, _ := userID(r)
	id, _ := strconv.Atoi(r.PathValue("id"))
	o, err := scanOrder(db.QueryRow(`SELECT `+orderCols+` FROM orders.orders WHERE id=$1 AND user_id=$2`, id, uid))
	if err != nil {
		common.Error(w, 404, "pedido no encontrado")
		return
	}
	common.JSON(w, 200, o)
}

// setStatus avanza el estado y agrega una entrada a la línea de tiempo del pedido.
func setStatus(orderID int, status, event, reason string) {
	_, err := db.Exec(`
		UPDATE orders.orders
		SET status=$2, reason=NULLIF($4,''),
		    timeline = timeline || jsonb_build_array(jsonb_build_object('status',$2::text,'event',$3::text,'at',now()))
		WHERE id=$1`, orderID, status, event, reason)
	if err != nil {
		log.Printf("✖ error actualizando pedido #%d: %v", orderID, err)
		return
	}
	log.Printf("pedido #%d → %s", orderID, status)
}

func main() {
	db = common.MustPostgres()
	nc = common.MustNATS("order-service")

	common.Subscribe(nc, common.SubjPaymentCompleted, "order-service", func(e common.PaymentResult) {
		setStatus(e.OrderID, "PAGADO", "PaymentCompleted", "")
	})
	common.Subscribe(nc, common.SubjPaymentFailed, "order-service", func(e common.PaymentResult) {
		setStatus(e.OrderID, "RECHAZADO", "PaymentFailed", e.Reason)
	})
	common.Subscribe(nc, common.SubjStockUpdated, "order-service", func(e common.StockUpdated) {
		setStatus(e.OrderID, "CONFIRMADO", "StockUpdated", "")
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", createOrder)
	mux.HandleFunc("GET /orders", listOrders)
	mux.HandleFunc("GET /orders/{id}", getOrder)
	common.Serve("order-service", common.Env("PORT", "8003"), mux)
}
