# Keel

## 1. Glossary

- **Task**: A task or job entity managed and executed by Keel. There are 3 types of task: `CronTask`, `DelayTask` and `UserTask`.
    - **CronTask**: A task or job being executed periodically, like Crontab in Linux. 
    - **DelayTask**: A onetime task or job that will be executed later after creation. For instance, a task that cancels unpaid orders created 30 minutes ago.  
    - **UserTask**: A task or job that is not executed either periodically or in the near future. Say you are going to build an application processing customers' media resources in background, such a process can be seen as `UserTask`, it is executed right after previous one finishes.
        - **TaskUid**: The user-provided unique identifier for an `UserTask`, must be unique under a `Tenant`.
- **TaskRun**: A record holding `Task` execution status, result, error and other details, created everytime a `Task` is executed. 
- **Tenant**: An entity representing a customer, a work team and so on. Keel schedules tasks per each tenant.
    - **TenantUid**: The user-provided unique identifier for a `Tenant`, must be unique across all tenants.
- **Zone**: Available zone. `Tenants` must be unique across multiple zones.
- **ResourceQuota**: Resource usage limit of a `Tenant`, e.g. CPU, memory, GPU, storage, concurrency or custom workload figure.
- **Scheduler**: The process that handles task scheduling, but does not execute tasks. There might be more than one `Scheduler` running but only one `Scheduler` (the leader) can dispatch tasks at one time.
- **Worker**: The process that executes tasks assigned by `Scheduler`.
