// User Service: registro, login y emisión/validación de JWT.
// El API Gateway (NGINX) llama a /auth/verify en cada petición protegida.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"agromarket/internal/common"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	db     *sql.DB
	secret = []byte(common.Env("JWT_SECRET", "agromarket-dev-secret"))
)

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func issueToken(id int, name, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   fmt.Sprint(id),
		"name":  name,
		"email": email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil || c.Email == "" || len(c.Password) < 4 || c.Name == "" {
		common.Error(w, 400, "nombre, email y contraseña (mín. 4 caracteres) son obligatorios")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	var id int
	err := db.QueryRow(`INSERT INTO users.accounts (name, email, password_hash) VALUES ($1, lower($2), $3) RETURNING id`,
		c.Name, c.Email, string(hash)).Scan(&id)
	if err != nil {
		common.Error(w, 409, "ese email ya está registrado")
		return
	}
	tok, _ := issueToken(id, c.Name, strings.ToLower(c.Email))
	common.JSON(w, 201, map[string]any{"token": tok, "user": map[string]any{"id": id, "name": c.Name, "email": c.Email}})
}

func login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		common.Error(w, 400, "JSON inválido")
		return
	}
	var id int
	var name, hash string
	err := db.QueryRow(`SELECT id, name, password_hash FROM users.accounts WHERE email = lower($1)`, c.Email).Scan(&id, &name, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		common.Error(w, 401, "credenciales incorrectas")
		return
	}
	tok, _ := issueToken(id, name, strings.ToLower(c.Email))
	common.JSON(w, 200, map[string]any{"token": tok, "user": map[string]any{"id": id, "name": name, "email": c.Email}})
}

func parse(r *http.Request) (jwt.MapClaims, error) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritmo inesperado")
		}
		return secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("token inválido")
	}
	return tok.Claims.(jwt.MapClaims), nil
}

// verify es usado por NGINX (auth_request). Devuelve 200 + cabeceras con la identidad.
func verify(w http.ResponseWriter, r *http.Request) {
	claims, err := parse(r)
	if err != nil {
		w.WriteHeader(401)
		return
	}
	w.Header().Set("X-User-Id", fmt.Sprint(claims["sub"]))
	w.Header().Set("X-User-Email", fmt.Sprint(claims["email"]))
	w.WriteHeader(200)
}

func me(w http.ResponseWriter, r *http.Request) {
	claims, err := parse(r)
	if err != nil {
		common.Error(w, 401, "no autenticado")
		return
	}
	common.JSON(w, 200, map[string]any{"id": claims["sub"], "name": claims["name"], "email": claims["email"]})
}

func main() {
	db = common.MustPostgres()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", register)
	mux.HandleFunc("POST /login", login)
	mux.HandleFunc("GET /me", me)
	mux.HandleFunc("GET /auth/verify", verify)
	common.Serve("user-service", common.Env("PORT", "8001"), mux)
}
