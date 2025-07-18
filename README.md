# Log Analytics System

This repository provides a small demo of a scalable log analytics platform written in Go. The system combines the following components:

- **Cassandra** – keeps project metadata such as API keys and TTL values.
- **CockroachDB** – stores user accounts (not fully implemented).
- **Kafka** – receives log events from the ingestion API.
- **ClickHouse** – persists log events for querying.

Three Go services are available:

1. **ingestion service** – accepts log events on port `8080` and publishes them to Kafka after validating the project API key in Cassandra. It also stores a small audit trail in CockroachDB.
2. **processor service** – consumes the Kafka messages and writes them to ClickHouse for querying.
3. **query service** – exposes endpoints on port `8082` to list event counts or retrieve event details filtered by arbitrary fields.

## Running the stack

Start everything with Docker Compose:

```sh
docker-compose up --build
```

The compose file launches Cassandra, CockroachDB, Kafka, ClickHouse and the three Go services.

## Sending a log event

```sh
curl -XPOST -d '{"project_id":"proj1","api_key":"k","name":"test","timestamp":"2024-01-01T00:00:00Z","fields":{"k":"v"}}' http://localhost:8080/
```

## Querying counts

```sh
curl 'http://localhost:8082?project=proj1'
```

To list recent events of a specific name and filter by field values:

```sh
curl 'http://localhost:8082/events?project=proj1&name=test&user=alice&limit=5'
```

These examples assume corresponding project data already exists in Cassandra.
