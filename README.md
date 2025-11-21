# Keel

Keel is a task scheduling and execution framework with multi-tenant support. It provides a gRPC + HTTP gateway API, Kafka-based transport between scheduler and workers, and pluggable database backends (MySQL or MongoDB, optionally with Redis).

Status: This project has not been validated in large-scale production. Evaluate carefully before adoption.

For Chinese, see `README_zh.md`.

## Concepts

- Tenant: Represents a customer or team. Scheduling is per-tenant.
  - TenantUid: User-provided unique identifier across all tenants.
  - Zone: Availability zone; tenants must remain unique across zones.
- Task: A job managed and executed by Keel (e.g., background media processing).
  - TaskUid: User-provided unique identifier under a tenant.
- ResourceQuota: Tenant resource limits such as CPU, memory, GPU, storage, concurrency or custom metrics.
- Scheduler: Handles scheduling only; one leader dispatches at a time.
- Worker: Executes tasks assigned by the scheduler.

## Features

- Multi-tenant scheduling and concurrency control
- Kafka-based transport
- Pluggable databases (MySQL/MongoDB, optional Redis)
- gRPC + HTTP gateway
- Scheduler state snapshots to S3-compatible storage (Minio)
- Node.js SDK (TypeScript)

## Architecture

- Scheduler: Manages tenant queues, concurrency and state; exposes APIs; dispatches via Kafka.
- Worker: Registers and executes handlers; reports status; supports retry and transition.
- Transport: Kafka for task dispatch and status reporting.
- Database: Tenant/task metadata in MySQL or MongoDB (Redis optional).
- Object Storage: Event snapshots via Minio.

API ports: HTTP uses `SCHEDULER_PORT`; gRPC uses `SCHEDULER_PORT + 1000`.

## Tech Stack

- Go (services), Node.js (SDK)
- gRPC, grpc-gateway, protobuf, OpenAPI
- Kafka (confluent-kafka-go)
- MySQL, MongoDB, Redis (optional)
- Minio (S3-compatible)
- zerolog, ants, bbolt

## Quick Start (Docker Compose)

Prerequisites: Docker and Docker Compose.

Build & up:

```
make integration-test
```

or:

```
docker-compose -f docker-compose.yaml up --build
```

Services:

- Kafka UI: `http://localhost:8080`
- Scheduler HTTP: `http://<SCHEDULER_ADDRESS>:<SCHEDULER_PORT>` (default `:8000`)
- Scheduler gRPC: `<SCHEDULER_ADDRESS>:<SCHEDULER_PORT+1000>` (default `:9000`)

## Run Natively (Go)

Set environment variables (see `tests/test.env`):

```
export KEEL_DATABASE_TYPE=mysql
export KEEL_DATABASE_DSN=root:pa88w0rd@tcp(mysql:3306)/keel?charset=utf8mb4&parseTime=true
export KEEL_SCHEDULER_ID=scheduler-1
export KEEL_SCHEDULER_ZONE=global
export KEEL_SCHEDULER_PORT=8000
export KEEL_TRANSPORT_TYPE=kafka
export KEEL_TRANSPORT_KAFKA_BROKERS=kafka:9092
export KEEL_TRANSPORT_KAFKA_TOPICS_TASKS=keel-tasks-0,keel-tasks-1
export KEEL_TRANSPORT_KAFKA_TOPICS_MESSAGES=keel-messages-0,keel-messages-1
```

Start scheduler:

```
go run ./tests/integration/scheduler
```

Start worker:

```
go run ./tests/integration/worker
```

## Configuration

Keel loads configuration from environment variables (prefix `KEEL_`). See `tests/test.env` for a complete example.

Key variables:

- Database: `KEEL_DATABASE_TYPE`, `KEEL_DATABASE_DSN`
- Scheduler: `KEEL_SCHEDULER_*` (`ID`, `ZONE`, `PORT`, `ADDRESS`, etc.)
- Snapshot: `KEEL_SNAPSHOT_*` (Minio connection)
- Worker: `KEEL_WORKER_*` (pool size, report interval)
- Transport (Kafka): `KEEL_TRANSPORT_*` (brokers, topics, groupId, TTL, SASL)

`configs/sample.yaml` is an early example; env vars are the primary source.

## Protocol & Codegen

Install protoc plugins:

```
make prepare
```

Generate code and OpenAPI:

```
make gen
```

Generated Go code is under `pkg/pb/*`; OpenAPI JSON under `docs/apidocs.swagger.json`.

## Node.js SDK

In `sdk/nodejs/`:

```
npm install
npm run build
```

Generates TypeScript/JS clients from `proto/*.proto` and builds them.

## Development & Testing

Unit tests:

```
make test
```

Integration environment:

```
make integration-test
make clear-integration-test
```

## License

See `LICENSE`.

## Project Structure (highlights)

- `pkg/scheduler`: Scheduler and API servers (gRPC/HTTP)
- `pkg/worker`: Worker core and handler registration
- `pkg/impl/transport`: Kafka transport implementation
- `pkg/impl/database`: Database implementations (MySQL/MongoDB/Redis combos)
- `pkg/config`: Config models and env parsing
- `pkg/pb`: Generated interface and gateway code from `proto/*.proto`
- `tests/integration`: Entrypoints and example handlers
- `docker-compose.yaml`, `Makefile`: Build and integration scripts
