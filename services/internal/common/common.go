// Package common agrupa utilidades compartidas por todos los microservicios:
// configuración, conexión a Postgres / Redis / NATS con reintentos,
// definición de eventos y helpers HTTP.
package common

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

// ---------- Eventos del Event Bus (NATS) ----------

const (
	SubjOrderCreated     = "orders.created"
	SubjPaymentCompleted = "payments.completed"
	SubjPaymentFailed    = "payments.failed"
	SubjStockUpdated     = "stock.updated"
)

type OrderItem struct {
	ProductID int     `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

// OrderCreated lo publica Order Service al crear un pedido.
type OrderCreated struct {
	OrderID   int         `json:"order_id"`
	UserID    int         `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Total     float64     `json:"total"`
	CreatedAt time.Time   `json:"created_at"`
}

// PaymentResult lo publica Payment Service (completed o failed).
type PaymentResult struct {
	OrderID   int         `json:"order_id"`
	UserID    int         `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Total     float64     `json:"total"`
	PaymentID string      `json:"payment_id,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

// StockUpdated lo publica Inventory Service tras descontar stock.
type StockUpdated struct {
	OrderID int          `json:"order_id"`
	UserID  int          `json:"user_id"`
	Changes []StockLevel `json:"changes"`
}

type StockLevel struct {
	ProductID int `json:"product_id"`
	Stock     int `json:"stock"`
}

// ---------- Configuración ----------

func Env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ---------- Infraestructura con reintentos ----------

func MustPostgres() *sql.DB {
	dsn := Env("DATABASE_URL", "postgres://agro:agro@localhost:5432/agromarket?sslmode=disable")
	var db *sql.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				log.Println("✔ conectado a PostgreSQL")
				return db
			}
		}
		log.Printf("esperando PostgreSQL... (%v)", err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("no se pudo conectar a PostgreSQL: %v", err)
	return nil
}

func MustRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: Env("REDIS_ADDR", "localhost:6379")})
	for i := 0; i < 30; i++ {
		if err := rdb.Ping(context.Background()).Err(); err == nil {
			log.Println("✔ conectado a Redis")
			return rdb
		} else {
			log.Printf("esperando Redis... (%v)", err)
		}
		time.Sleep(2 * time.Second)
	}
	log.Fatal("no se pudo conectar a Redis")
	return nil
}

func MustNATS(name string) *nats.Conn {
	url := Env("NATS_URL", nats.DefaultURL)
	for i := 0; i < 30; i++ {
		nc, err := nats.Connect(url, nats.Name(name), nats.MaxReconnects(-1))
		if err == nil {
			log.Println("✔ conectado a NATS (Event Bus)")
			return nc
		}
		log.Printf("esperando NATS... (%v)", err)
		time.Sleep(2 * time.Second)
	}
	log.Fatal("no se pudo conectar a NATS")
	return nil
}

// Publish serializa y publica un evento en el bus.
func Publish(nc *nats.Conn, subject string, v any) {
	data, _ := json.Marshal(v)
	if err := nc.Publish(subject, data); err != nil {
		log.Printf("✖ error publicando %s: %v", subject, err)
		return
	}
	log.Printf("→ evento publicado [%s] %s", subject, string(data))
}

// Subscribe se suscribe con queue group (permite escalar réplicas del consumidor).
func Subscribe[T any](nc *nats.Conn, subject, queue string, handler func(T)) {
	_, err := nc.QueueSubscribe(subject, queue, func(m *nats.Msg) {
		var evt T
		if err := json.Unmarshal(m.Data, &evt); err != nil {
			log.Printf("✖ evento inválido en %s: %v", subject, err)
			return
		}
		log.Printf("← evento recibido [%s]", subject)
		handler(evt)
	})
	if err != nil {
		log.Fatalf("no se pudo suscribir a %s: %v", subject, err)
	}
}

// ---------- HTTP helpers ----------

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// Logging middleware sencillo.
func Logging(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s (%s)", service, r.Method, r.URL.Path, time.Since(start))
	})
}

// Serve arranca el servidor HTTP con /health incluido.
func Serve(service, port string, mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, 200, map[string]string{"service": service, "status": "ok"})
	})
	log.Printf("🚀 %s escuchando en :%s", service, port)
	log.Fatal(http.ListenAndServe(":"+port, Logging(service, mux)))
}
