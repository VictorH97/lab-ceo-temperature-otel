FROM golang:latest as builder
WORKDIR /cep
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -C cmd/server-validate-cep -ldflags="-w -s" -o ../../server-validate-cep

FROM scratch
WORKDIR /cep
COPY --from=builder /cep/server-validate-cep .
ENTRYPOINT ["./server-validate-cep"]