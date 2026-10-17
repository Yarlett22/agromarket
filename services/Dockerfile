# Imagen única multi-servicio: se elige qué microservicio compilar con --build-arg SERVICE=...
FROM golang:1.23-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=America/Lima
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
