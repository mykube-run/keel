# Keel

## 1. Glossary

- **Tenant**: An entity representing a customer, a work team and so on. Keel schedules tasks per each tenant.
    - **TenantUid**: The user-provided unique identifier for a `Tenant`, must be unique across all tenants.
    - **Zone**: Available zone. `Tenants` must be unique across multiple zones.
- **Task**: A task or job entity managed and executed by Keel. Say you are going to build an application processing customers' media resources in background, such a process can be seen as `Task`.
    - **TaskUid**: The user-provided unique identifier for `Task`, must be unique under a `Tenant`.
- **ResourceQuota**: Resource usage limit of a `Tenant`, e.g. CPU, memory, GPU, storage, concurrency or custom workload figure.
- **Scheduler**: The process that handles task scheduling, but does not execute tasks. There might be more than one `Scheduler` running but only one `Scheduler` (the leader) can dispatch tasks at one time.
- **Worker**: The process that executes tasks assigned by `Scheduler`.
