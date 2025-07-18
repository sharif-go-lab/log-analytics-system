# Log Analytics System

This project provides a small log analytics system demonstrating the use of
multiple distributed databases.  It exposes a very small web UI/API to ingest
and query events.

The stack uses the following technologies:

- **Cassandra** – storing log metadata and project information.
- **Kafka** – ingestion pipeline for logs.
- **ClickHouse** – analytical storage for querying events.
- **CockroachDB** – storing authentication and user data.
- **Go** – main application server and consumers.

The system architecture is designed around containers defined in `docker-compose.yml`.

See `DESIGN.md` for a more detailed explanation of the chosen architecture in Persian.

## Running the stack

```bash
# Build and start all services
$ docker-compose up --build
```

The Go application exposes HTTP endpoints for:

- `/login` – authenticate a user stored in CockroachDB.
- `/projects` – create or list projects. Creating a project returns an API key
  that must be used when pushing logs.
- `/log` – submit a log message for a project. The message is put on Kafka and
  later stored in Cassandra (with TTL) and ClickHouse.
- `/projects/{id}/events` – query recent events for a project from ClickHouse.
- `/projects/{id}/events/{name}` – fetch the latest event of a given name.

Logs are produced to Kafka and a background consumer moves them into Cassandra
(for short term storage with TTL) and ClickHouse (for analytics).

`docker-compose.yml` starts a small cluster with replication factor three for
Cassandra, Kafka brokers and CockroachDB nodes. ClickHouse runs as a single
instance for simplicity.

## Generating test logs

A small command line generator is available in the `generator` directory.
Build it with:

```bash
$ cd generator
$ go build
```

Run it by providing the project ID and API key obtained from the `/projects`
endpoint:

```bash
$ ./generator -project <id> -key <api_key> -n 500
```

This will produce sample log events to the running server.
