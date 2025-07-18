FROM golang:1.21-alpine AS builder
WORKDIR /src
COPY . .
RUN go build -o /bin/ingestion ./cmd/ingestion
RUN go build -o /bin/query ./cmd/query
RUN go build -o /bin/processor ./cmd/processor
RUN go build -o /bin/web ./cmd/web

FROM alpine:latest
COPY --from=builder /bin/ingestion /ingestion
COPY --from=builder /bin/query /query
COPY --from=builder /bin/processor /processor
COPY --from=builder /bin/web /web
CMD ["/ingestion"]
