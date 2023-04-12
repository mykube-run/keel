CREATE TABLE IF NOT EXISTS tenant
(
    uid            VARCHAR(255) NOT NULL PRIMARY KEY COMMENT 'Tenant uid',
    zone           VARCHAR(20)  NOT NULL DEFAULT 'default' COMMENT 'Tenant zone',
    priority       INT          NOT NULL COMMENT 'Tenant priority',
    partition_name VARCHAR(50)  NOT NULL COMMENT 'Partition this tenant belongs to',
    name           VARCHAR(100) NOT NULL COMMENT 'Tenant name',
    status         VARCHAR(20)  NOT NULL COMMENT 'Tenant status, possible values are: Active, Inactive',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the tenant was created at',
    updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'UTC time the tenant was updated at',
    last_active    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the tenant was last active',
    INDEX idx_zone_partition (zone, partition_name)
) ENGINE = InnoDB COMMENT 'Tenant';

CREATE TABLE IF NOT EXISTS resourcequota
(
    tenant_id   VARCHAR(255) NOT NULL PRIMARY KEY COMMENT 'Tenant uid',
    concurrency INT   DEFAULT 0 COMMENT 'Task concurrency quota',
    cpu         INT   DEFAULT 0 COMMENT 'CPU quota in cores',
    custom      INT   DEFAULT 0 COMMENT 'Custom resource quota',
    gpu         INT   DEFAULT 0 COMMENT 'GPU quota in cores',
    memory      INT   DEFAULT 0 COMMENT 'Memory quota in MB',
    storage     INT   DEFAULT 0 COMMENT 'Storage quota in MB',
    peak        FLOAT DEFAULT 0 COMMENT 'Actual or realtime resource usage can exceed quota in percent. Ranged from 0 to 1.0 (or higher)'
)
    ENGINE = InnoDB COMMENT 'Tenant resource quota options. Resource quotas are optional, 0 means there is no limit for specified resource type';

CREATE TABLE IF NOT EXISTS task
(
    uid               VARCHAR(255) NOT NULL PRIMARY KEY COMMENT 'Task uid',
    tenant_id         VARCHAR(255) NOT NULL COMMENT 'Tenant uid',
    handler           VARCHAR(100) NOT NULL COMMENT 'Task handler',
    config            JSON         NOT NULL COMMENT 'Task config',
    schedule_strategy VARCHAR(20)  NOT NULL COMMENT 'Task schedule strategy',
    priority          INT          NOT NULL COMMENT 'Task priority',
    progress          INT          NOT NULL DEFAULT 0 COMMENT 'Task progress, can be used as break point to continue processing',
    status            VARCHAR(20)  NOT NULL COMMENT 'Task status, possible values are: Pending, Scheduling, Dispatched, Running, NeedsRetry, InTransition, Success, Failed, Canceled',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the task was created at',
    updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'UTC time the task was updated at',
    INDEX idx_handler (handler)
) ENGINE = InnoDB COMMENT 'Tasks';

CREATE TABLE IF NOT EXISTS taskrun
(
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id     VARCHAR(255) NOT NULL COMMENT 'Tenant uid',
    task_id       VARCHAR(255) NOT NULL COMMENT 'Task uid',
    task_type     VARCHAR(20)  NOT NULL COMMENT 'Task type, available options are: CronTask, DelayTask, UserTask',
    schedule_type VARCHAR(20)  NOT NULL COMMENT 'Task schedule type, maybe firstrun, retry or continue',
    status        VARCHAR(20)  NOT NULL COMMENT 'Task run status',
    result        VARCHAR(255) NULL COMMENT 'Task run result',
    error         VARCHAR(255) NULL COMMENT 'Task encountered error (if there is any)',
    start         DATETIME     NULL COMMENT 'UTC time the stask started running',
    end           DATETIME     NULL COMMENT 'UTC time the task finished/exited',
    progress      INT          NULL COMMENT 'Task run progress',
    INDEX idx_tenant_id_task_type_task_id (tenant_id, task_type, task_id),
    INDEX idx_tenant_id_start_task_type (tenant_id, start, task_type),
    INDEX idx_tenant_id_end_task_type (tenant_id, end, task_type)
) ENGINE = InnoDB COMMENT 'Task run history';

CREATE TABLE IF NOT EXISTS taskrunlog
(
    id  BIGINT PRIMARY KEY COMMENT 'Task run id',
    log TEXT COMMENT 'Log'
) ENGINE = InnoDB COMMENT 'Task run log';