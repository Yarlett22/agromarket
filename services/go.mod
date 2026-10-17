module agromarket

go 1.23

replace golang.org/x/crypto => github.com/golang/crypto v0.27.0

replace golang.org/x/sys => github.com/golang/sys v0.25.0

replace golang.org/x/text => github.com/golang/text v0.18.0

replace golang.org/x/net => github.com/golang/net v0.29.0

require (
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/lib/pq v1.10.9
	github.com/nats-io/nats.go v1.37.0
	github.com/redis/go-redis/v9 v9.7.0
	golang.org/x/crypto v0.27.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/sys v0.25.0 // indirect
)
