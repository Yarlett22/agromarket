# 1) Build con Webpack 5
FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN npm run build

# 2) Servir estáticos con NGINX (el API Gateway hace proxy hacia este contenedor)
FROM nginx:1.27-alpine
COPY nginx-web.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
