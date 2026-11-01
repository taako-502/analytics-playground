FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY cmd/server ./cmd/server
COPY internal/pipeline ./internal/pipeline
RUN CGO_ENABLED=0 go build -o /server ./cmd/server
FROM alpine:3.21
RUN apk add --no-cache postgresql-client
WORKDIR /app
COPY --from=build /server /app/server
COPY web /app/web
COPY sql /app/sql
EXPOSE 8080
CMD ["/app/server"]
