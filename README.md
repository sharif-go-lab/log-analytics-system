# Log Analytics System

This repository contains a small generator program that can be used to produce
sample data for the ingestion API. The generator sends repeated POST requests
to a configured endpoint with simple JSON payloads.

## Building the Generator

```
# from the repository root
cd cmd/generator
go build -o generator
```

## Running the Generator

Run the program and specify the URL of your ingestion API. By default, the
program sends a request every second to `http://localhost:8080/ingest`.

```
./generator -url http://localhost:8080/ingest -interval 1s
```

Flags:

- `-url` — address of the ingestion API endpoint.
- `-interval` — time between requests (for example `500ms`, `2s`).

The program will keep running until stopped and log the status of each
request. This can be useful for generating test data while developing the
ingestion service.

