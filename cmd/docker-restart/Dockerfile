FROM golang:latest AS builder
WORKDIR /build

COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o docker-restart cmd/docker-restart/main.go

FROM scratch
COPY --from=builder /build/docker-restart /docker-restart
ENTRYPOINT ["/docker-restart"]
