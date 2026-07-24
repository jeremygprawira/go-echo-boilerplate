# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/http

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/server /app/server
COPY config/ /app/config/
COPY api-docs.html /app/api-docs.html
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/server"]
