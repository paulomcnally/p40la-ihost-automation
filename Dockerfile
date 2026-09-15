# Stage 1: Build frontend (React + Vite + Tailwind)
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# Stage 2: Build backend (Go)
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copiar el build del frontend al public/ del backend
COPY --from=frontend-builder /app/public ./public

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Stage 3: Runtime (distroless)
FROM gcr.io/distroless/static-debian12
WORKDIR /app

ENV DATA_DIR=/app/data
ENV PORT=8089

COPY --from=backend-builder /app/server /app/server
COPY --from=backend-builder /app/public /app/public
COPY --from=backend-builder /app/migrations /app/migrations
EXPOSE 8089
VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/server", "-healthcheck"]

CMD ["/app/server"]
