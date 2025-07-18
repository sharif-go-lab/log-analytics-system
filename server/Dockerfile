FROM golang:1.20 as builder
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN go build -o server

FROM gcr.io/distroless/base-debian11
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8081
CMD ["./server"]
