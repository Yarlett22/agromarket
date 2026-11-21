// User Service: integración con Keycloak (servidor de identidad).
//
//   - Keycloak autentica al usuario (login, registro, contraseñas) y emite un
//     access token JWT firmado con RS256 (llave privada que solo tiene Keycloak).
//   - Este servicio descarga las llaves PÚBLICAS de Keycloak (JWKS) y valida la
//     firma, la expiración, el emisor (iss) y el cliente (azp) de cada token.
//   - El API Gateway (NGINX) llama a /auth/verify en cada petición protegida
//     (auth_request) y reenvía la identidad a los microservicios en X-User-Id.
//   - Mantiene un perfil local (users.accounts) enlazado al "sub" de Keycloak.
package main

import (
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"agromarket/internal/common"

	"github.com/golang-jwt/jwt/v5"
)

var (
	db       *sql.DB
	realmURL = common.Env("KEYCLOAK_REALM_URL", "http://keycloak:8080/auth/realms/agromarket") // red interna
	clientID = common.Env("KEYCLOAK_CLIENT_ID", "agromarket-web")
	realm    = common.Env("KEYCLOAK_REALM", "agromarket")
	ids      sync.Map // sub de Keycloak → id local (caché)
)

// ---------------- JWKS: llaves públicas de Keycloak ----------------

type jwks struct {
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

var keyset = &jwks{keys: map[string]*rsa.PublicKey{}}

func (k *jwks) refresh() error {
	resp, err := http.Get(realmURL + "/protocol/openid-connect/certs")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var doc struct {
		Keys []struct {
			Kid, Kty, Alg, Use, N, E string
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, j := range doc.Keys {
		if j.Kty != "RSA" || j.Use == "enc" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(j.N)
		e, err2 := base64.RawURLEncoding.DecodeString(j.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[j.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return errors.New("Keycloak no devolvió llaves RSA")
	}
	k.mu.Lock()
	k.keys, k.fetched = keys, time.Now()
	k.mu.Unlock()
	log.Printf("🔑 JWKS de Keycloak cargado (%d llave(s) RS256)", len(keys))
	return nil
}

func (k *jwks) get(kid string) (*rsa.PublicKey, error) {
	k.mu.RLock()
	key, ok := k.keys[kid]
	stale := time.Since(k.fetched) > time.Minute
	k.mu.RUnlock()
	if ok {
		return key, nil
	}
	if !stale && len(k.keys) > 0 {
		return nil, fmt.Errorf("kid %q desconocido", kid)
	}
	if err := k.refresh(); err != nil { // rotación de llaves o primer uso
		return nil, err
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	if key, ok := k.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("kid %q desconocido", kid)
}

// ---------------- Validación del token ----------------

type claims struct {
	jwt.RegisteredClaims
	Azp               string `json:"azp"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

func parse(r *http.Request) (*claims, error) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if raw == "" {
		return nil, errors.New("sin token")
	}
	c := &claims{}
	_, err := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return keyset.get(kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithExpirationRequired(), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return nil, err
	}
	// El emisor depende del host público (localhost, Codespaces...), pero siempre
	// termina en /realms/<realm>; el token debe haber sido emitido para nuestro cliente.
	if !strings.HasSuffix(c.Issuer, "/realms/"+realm) {
		return nil, fmt.Errorf("emisor no confiable: %s", c.Issuer)
	}
	if c.Azp != clientID {
		return nil, fmt.Errorf("token emitido para otro cliente: %s", c.Azp)
	}
	return c, nil
}

// localID enlaza el usuario de Keycloak con un id numérico interno (perfil local).
func localID(c *claims) (int, error) {
	if v, ok := ids.Load(c.Subject); ok {
		return v.(int), nil
	}
	name := c.Name
	if name == "" {
		name = c.PreferredUsername
	}
	var id int
	err := db.QueryRow(`
		INSERT INTO users.accounts (keycloak_id, name, email) VALUES ($1, $2, $3)
		ON CONFLICT (keycloak_id) DO UPDATE SET name = EXCLUDED.name, email = EXCLUDED.email, last_login = now()
		RETURNING id`, c.Subject, name, c.Email).Scan(&id)
	if err != nil {
		return 0, err
	}
	ids.Store(c.Subject, id)
	log.Printf("👤 usuario de Keycloak %s (%s) → id local %d", c.PreferredUsername, c.Subject, id)
	return id, nil
}

// verify es usado por NGINX (auth_request): 200 + identidad, o 401.
func verify(w http.ResponseWriter, r *http.Request) {
	c, err := parse(r)
	if err != nil {
		log.Printf("✖ token rechazado: %v", err)
		w.WriteHeader(401)
		return
	}
	id, err := localID(c)
	if err != nil {
		log.Printf("✖ perfil local: %v", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("X-User-Id", fmt.Sprint(id))
	w.Header().Set("X-User-Email", c.Email)
	w.Header().Set("X-User-Roles", strings.Join(c.RealmAccess.Roles, ","))
	w.WriteHeader(200)
}

// me devuelve el perfil del usuario autenticado y los datos del token.
func me(w http.ResponseWriter, r *http.Request) {
	c, err := parse(r)
	if err != nil {
		common.Error(w, 401, "no autenticado")
		return
	}
	id, _ := localID(c)
	common.JSON(w, 200, map[string]any{
		"id": id, "keycloak_id": c.Subject, "username": c.PreferredUsername, "name": c.Name, "email": c.Email,
		"roles": c.RealmAccess.Roles, "issuer": c.Issuer, "algorithm": "RS256", "expires_at": c.ExpiresAt.Time,
	})
}

func main() {
	db = common.MustPostgres()
	go func() { // precarga del JWKS (Keycloak puede tardar en arrancar)
		for i := 0; i < 60; i++ {
			if err := keyset.refresh(); err == nil {
				return
			}
			time.Sleep(3 * time.Second)
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /me", me)
	mux.HandleFunc("GET /auth/verify", verify)
	common.Serve("user-service", common.Env("PORT", "8001"), mux)
}
