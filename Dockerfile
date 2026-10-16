# Stage 1: Build Frontend Assets
FROM node:24-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Backend Binaries
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bin/cuan-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bin/cuan-runner ./cmd/runner
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bin/cuan-migrate ./cmd/migrate

# Stage 3: Minimal Production Image
FROM alpine:3.20 AS runner
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 1000 cuan && \
    adduser -u 1000 -G cuan -s /bin/sh -D cuan

WORKDIR /app
COPY --from=backend-builder /app/bin/cuan-server /app/cuan-server
COPY --from=backend-builder /app/bin/cuan-runner /app/cuan-runner
COPY --from=backend-builder /app/bin/cuan-migrate /app/cuan-migrate
COPY --from=frontend-builder /app/web/dist /app/web/dist
COPY migrations /app/migrations

ENV PORT=8080
ENV CUAN_ENV=production
ENV CUAN_STATIC_DIR=/app/web/dist
ENV CUAN_AUTO_MIGRATE=true

USER cuan:cuan
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/cuan-server", "--healthcheck"]

CMD ["/app/cuan-server"]
