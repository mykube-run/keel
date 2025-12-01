# Keel

 Keel 是一个支持多租户的任务调度与执行框架。对外提供 gRPC 与 HTTP（grpc-gateway）接口，调度器与工作节点之间可通过 Kafka 或 gRPC 进行通信，底层存储可选择 MySQL 或 MongoDB（可搭配 Redis）。

状态说明：本项目尚未经过大规模生产验证，使用前请谨慎评估。

English README: `README.md`。

## 概念（Concepts）

- 租户（Tenant）：客户或团队的实体，调度按租户维度进行。
  - TenantUid：用户自定义的租户唯一标识，跨所有租户必须唯一。
  - Zone：可用区，租户在不同可用区也必须保持唯一。
- 任务（Task）：由 Keel 管理与执行的任务或作业，例如后台处理媒体资源。
  - TaskUid：用户自定义的任务唯一标识，在同一租户下必须唯一。
- 资源配额（ResourceQuota）：限制租户资源使用的上限，如 CPU、内存、GPU、存储、并发或自定义指标。
- 调度器（Scheduler）：负责任务调度但不执行任务；同一时间仅有一个调度器（Leader）进行任务派发。
- 工作节点（Worker）：执行由调度器分配的任务。

## 特性（Features）

- 多租户调度与并发控制
- 基于 Kafka 的消息传输
- 可插拔数据库实现（MySQL、MongoDB，支持 Redis 组合）
- gRPC 与 HTTP 网关接口
- 将调度器状态快照保存至 S3 兼容存储（Minio）
- 提供 Node.js SDK（TypeScript）

## 架构（Architecture）

- 调度器：维护租户队列、并发与状态机，暴露 API；通过 Kafka 下发任务与接收回报。
- 工作节点：注册并执行任务处理器，周期上报状态，支持重试与迁移。
 - 传输层：Kafka 或 gRPC 作为任务与状态消息通道。
- 数据库层：任务/租户元数据存储（MySQL 或 MongoDB，可选 Redis 辅助）。
- 对象存储：用于事件快照（Minio）。

 API 端口：HTTP 使用 `SCHEDULER_PORT`，gRPC 使用 `SCHEDULER_PORT + 1000`。

## 调度器与工作节点通信

- 传输选项
  - `Kafka`：调度器将任务下发到 Kafka 主题；工作节点从任务主题消费并将状态回报到消息主题。
  - `gRPC`：通过 `pb.Transport.Connect` 建立双向流。
    - 调度器作为服务端，校验入站流（`x-api-key` 或 `Auth.Type=simple`）。
    - 工作节点作为客户端，发现并维护到所有调度器的并行连接。

- gRPC（Worker 侧）
  - 发现模式：`static`（端点列表）、`dns`（A 记录）、`k8s`（服务 DNS）。`dns`/`k8s` 模式下端口由 `GrpcConfig.Port` 指定（默认 `443`）。
  - 建连后，调度器在响应头返回自身标识 `x-identifier`；工作节点据此建立 `schedulerId → stream` 路由映射。
  - 心跳：周期广播 `to="__heartbeat"` 到所有已连接调度器，汇报当前处理器（handlers）。
  - 发送路由：状态/事件消息使用 `to="<SchedulerId>:<TaskId>"`，工作节点按 `SchedulerId` 选择对应调度器连接发送。

- gRPC（Scheduler 侧）
  - 接收工作节点流，记录 `workerId` 与处理器集合；维护 `handler → workers` 映射用于任务下发。
  - 派发任务时根据 `task.handler` 选择支持该处理器的在线工作节点连接并发送。
  - 在 `to="__heartbeat"` 的心跳消息中更新处理器映射与存活状态。

- 安全
  - TLS：通过 `GrpcConfig.TLSEnable` 开启。
  - mTLS：调度器加载 `TLSCertFile/TLSKeyFile` 与 `TLSCAFile`（校验客户端证书）；工作节点可加载客户端证书/密钥。
  - API Key：工作节点发送 `x-api-key`；调度器通过 `GrpcConfig.APIKey` 或当 `Auth.Type="simple"` 时使用 `Auth.APIKeys` 验证。

## 技术栈（Tech Stack）

- Go（主服务）、Node.js（SDK）
- gRPC、grpc-gateway、protobuf、OpenAPI
- Kafka（confluent-kafka-go）
- MySQL、MongoDB、Redis（可选）
- Minio（S3 兼容）
- zerolog、ants、bbolt

## 快速开始（Docker Compose）

前置条件：已安装 Docker 与 Docker Compose。

构建并启动：

```
make integration-test
```

或：

```
docker-compose -f docker-compose.yaml up --build
```

相关服务：

- Kafka UI: `http://localhost:8080`
- Scheduler HTTP: `http://<SCHEDULER_ADDRESS>:<SCHEDULER_PORT>`（默认 `:8000`）
- Scheduler gRPC: `<SCHEDULER_ADDRESS>:<SCHEDULER_PORT+1000>`（默认 `:9000`）

## 本地原生运行（Go）

设置环境变量（参考 `tests/test.env`）：

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

启动调度器：

```
go run ./tests/integration/scheduler
```

启动工作节点：

```
go run ./tests/integration/worker
```

## 配置（Configuration）

Keel 通过环境变量加载配置（前缀 `KEEL_`）。完整示例见 `tests/test.env`。

主要变量：

- 数据库：`KEEL_DATABASE_TYPE`、`KEEL_DATABASE_DSN`
- 调度器：`KEEL_SCHEDULER_*`（`ID`、`ZONE`、`PORT`、`ADDRESS` 等）
- 快照：`KEEL_SNAPSHOT_*`（Minio 连接）
- 工作节点：`KEEL_WORKER_*`（线程池、上报周期）
- 传输（Kafka）：`KEEL_TRANSPORT_*`（brokers、topics、groupId、TTL、SASL）

`configs/sample.yaml` 为早期示例，实际以环境变量为准。

## 协议与代码生成（Protocol & Codegen）

安装插件：

```
make prepare
```

生成代码与 OpenAPI：

```
make gen
```

生成的 Go 代码位于 `pkg/pb/*`，OpenAPI 文档位于 `docs/apidocs.swagger.json`。

## Node.js SDK

在 `sdk/nodejs/` 中：

```
npm install
npm run build
```

将根据 `proto/*.proto` 生成 TypeScript/JS 客户端并编译。

## 开发与测试（Development & Testing）

单元测试：

```
make test
```

集成测试环境：

```
make integration-test
make clear-integration-test
```

## 许可证（License）

见 `LICENSE`。

## 项目结构（要点）

- `pkg/scheduler`：调度器与 API 服务器（gRPC/HTTP）
- `pkg/worker`：工作节点核心逻辑与处理器注册
- `pkg/impl/transport`：Kafka 传输实现
- `pkg/impl/database`：数据库实现（MySQL/MongoDB/Redis 组合）
- `pkg/config`：配置模型与环境变量解析
- `pkg/pb`：由 `proto/*.proto` 生成的接口与网关代码
- `tests/integration`：调度器与工作节点入口与示例处理器
- `docker-compose.yaml`、`Makefile`：构建与集成运行脚本
