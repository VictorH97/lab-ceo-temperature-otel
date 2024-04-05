FROM golang:latest as builder
WORKDIR /weather
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -C cmd/server-find-temperature -ldflags="-w -s" -o ../../server-find-temperature

FROM scratch
WORKDIR /weather
COPY --from=builder /weather/server-find-temperature .
ENTRYPOINT ["./server-find-temperature"]