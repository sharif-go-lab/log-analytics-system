FROM golang:1.21-alpine AS builder
WORKDIR /src
COPY . .
RUN go build -o /bin/ingestion ./cmd/ingestion
RUN go build -o /bin/query ./cmd/query
RUN go build -o /bin/processor ./cmd/processor

FROM alpine:latest
COPY --from=builder /bin/ingestion /ingestion
COPY --from=builder /bin/query /query
COPY --from=builder /bin/processor /processor
CMD ["/ingestion"]
