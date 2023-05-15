ARG alpine_ver=latest
ARG golang_ver=1.20-alpine
FROM library/golang:$golang_ver AS builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -ldflags="-extldflags=-static" -o /usr/local/bin/gateway-node-controller ./cmd/gateway-node-controller


FROM scratch

COPY --from=builder /usr/local/bin/gateway-node-controller /bin/gateway-node-controller

ENTRYPOINT ["/bin/gateway-node-controller"]
