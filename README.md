# Log Analytics System

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](go.mod)

A small log analytics platform written in Go. Applications send events over HTTP. The events go through Kafka into ClickHouse, where they can be counted and searched by project, event name and custom fields, either through a JSON API or a web interface. Each project has its own API key and retention period, and ClickHouse drops events once their retention expires.

Everything, including Cassandra, CockroachDB, Kafka and ClickHouse, runs with a single `docker compose up`.

## Architecture

```mermaid
flowchart LR
    App["your app / cmd/generator"] -->|"POST / (JSON)"| ING["ingestion :8080"]
    ING -->|"check project + API key"| CAS[("Cassandra<br/>projects")]
    ING -->|"audit row"| CRDB[("CockroachDB<br/>users, audit")]
    ING -->|"publish"| K[["Kafka topic logs"]]
    K --> PROC["processor"]
    PROC -->|"insert"| CH[("ClickHouse<br/>events, with TTL")]
    Q["query API :8082"] --> CH
    Q --> CAS
    WEB["web UI :8083"] --> CH
    WEB --> CAS
    WEB --> CRDB
```

| Service | Port | What it does |
|---|---|---|
| `ingestion` (`cmd/ingestion`) | 8080 | Accepts events, checks the project ID and API key in Cassandra, computes the event's expiry from the project's TTL, publishes it to Kafka and writes an audit row to CockroachDB. |
| `processor` (`cmd/processor`) | none | Reads events from Kafka as consumer group `processor` and inserts them into ClickHouse. |
| `query` (`cmd/query`) | 8082 | JSON API for event counts and event lists, with filters on the project's search keys. |
| `web` (`cmd/web`) | 8083 | HTML pages for signing in, creating projects and browsing events. |
| `generator` (`cmd/generator`) | none | Command-line tool that sends a random event every second, for testing. |

Each store has one job:

- Cassandra holds projects: ID, API key, TTL in seconds, owner and the list of searchable field names.
- CockroachDB holds web UI users (bcrypt-hashed passwords) and the audit trail of ingested events.
- Kafka separates ingestion from storage, so a slow or restarting ClickHouse doesn't block incoming events.
- ClickHouse stores the events in a `MergeTree` table partitioned by project and ordered by `(project_id, name, timestamp)`. Its `TTL toDateTime(expire_at)` clause deletes expired rows in the background.

The services create their keyspace, databases and tables on startup. They retry each database connection 30 times, 3 seconds apart, so the stack can start in any order.

The design notes (in Persian) are in [DesignDoc.md](DesignDoc.md).

## Quick start

You need Docker with Compose.

```bash
docker compose up --build
```

1. Open http://localhost:8083/login and sign in as `admin` / `admin`. This user is created on first start, and you can register more users at `/register`.
2. Go to http://localhost:8083/projects/new and create a project. Give it an ID, a TTL in seconds (how long its events are kept) and the field names you want to search by, separated by commas, for example `user,level`. The next page shows the project's API key.
3. Send events with that project ID and key:

   ```bash
   curl -X POST http://localhost:8080/ \
     -d '{"project_id":"shop","api_key":"<key>","name":"purchase","timestamp":"2025-07-24T12:00:00Z","fields":{"user":"alice","level":"info"}}'
   ```

   A valid event gets `202 Accepted`. An unknown project or a wrong key gets `401`.

   Or let the generator send one event per second:

   ```bash
   INGEST_URL=http://localhost:8080/ PROJECT_ID=shop API_KEY=<key> go run ./cmd/generator
   ```

   Its events are named `login`, `purchase` or `error`, with random `user` (`alice`, `bob`, `carol`) and `level` (`info`, `warn`, `error`) fields.

4. Browse the events at http://localhost:8083/projects, or use the query API below.

## Event format

| Field | Type | Notes |
|---|---|---|
| `project_id` | string | Must exist in Cassandra. |
| `api_key` | string | Must match the project's key. |
| `name` | string | Event name, used for grouping. |
| `timestamp` | RFC 3339 time | Stored as Unix seconds. The event expires at `timestamp` + the project's TTL. |
| `fields` | object of strings | Any key/value pairs. Only keys listed as the project's search keys can be filtered on. |

## Query API

All requests need `project`. Any other query parameter whose name is one of the project's search keys becomes an exact-match filter on that field; other parameters are ignored. Expired events are never returned.

Event counts per name, with the latest timestamp of each:

```bash
curl 'http://localhost:8082/?project=shop'
curl 'http://localhost:8082/?project=shop&level=error'
```

```json
[{"Name":"purchase","Count":42,"Last":1753358400}]
```

The most recent events, optionally for one event name (`limit` defaults to 10):

```bash
curl 'http://localhost:8082/events?project=shop&name=purchase&user=alice&limit=5'
```

```json
[{"Name":"purchase","Timestamp":1753358400,"Fields":{"level":"info","user":"alice"}}]
```

## Web interface

| Page | What it shows |
|---|---|
| `/login`, `/register`, `/logout` | Sign in, create an account, sign out. |
| `/projects` | All projects, with a link to create a new one. |
| `/projects/new` | Form for a new project (ID, TTL, search keys). Shows the generated 8-character API key. |
| `/events?project=<id>` | Counts per event name and the latest timestamp, with one filter box per search key. |
| `/event?project=<id>&name=<name>` | One event with all its fields, and Prev / Next links to step through the matching events. |

## Configuration

The services read these environment variables. `docker-compose.yml` sets them for the containers.

| Variable | Used by | Value in `docker-compose.yml` |
|---|---|---|
| `CASSANDRA_HOST`, `CASSANDRA_KEYSPACE` | ingestion, query, web | `cassandra`, `logs` |
| `COCKROACH_DSN` | ingestion, web | `postgres://root@cockroach:26257/defaultdb?sslmode=disable` |
| `KAFKA_BROKERS` | ingestion, processor | `kafka:9092` (comma-separated list for several brokers) |
| `KAFKA_TOPIC` | ingestion, processor | `logs` |
| `CLICKHOUSE_DSN` | processor, query, web | `tcp://username:password@clickhouse:9000?database=logs` |
| `INGEST_URL`, `PROJECT_ID`, `API_KEY` | generator | not set; defaults are `http://localhost:8080/`, `proj1`, `k` |

Other ports published by Compose: Cassandra 9042, CockroachDB 26257 (with its admin UI on 8081), Kafka 9092, ZooKeeper 2181, ClickHouse 8123 (HTTP) and 9000 (native).

## Project layout

```
cmd/
  ingestion/  processor/  query/  web/     the four services
  generator/                               test event sender
internal/
  app/ingestion/   HTTP handler: validate, publish, audit
  app/processor/   Kafka to ClickHouse loop
  app/query/       counts and event list endpoints
  app/web/         HTML pages and cookie sign-in
  db/cassandra/    projects table
  db/cockroach/    users and audit tables, bcrypt
  db/clickhouse/   events table, TTL, filtered queries
  kafka/           producer and consumer (segmentio/kafka-go)
templates/         HTML templates for the web UI
docs/diagram.mmd   architecture diagram
Dockerfile         one image with all four service binaries
```

## Authors

- [Kasra Siavashpour](https://github.com/kasra-sia)
- [Ardalan Siavashpour](https://github.com/Ardalan-Sia)
