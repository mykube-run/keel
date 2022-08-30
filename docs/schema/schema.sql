CREATE TABLE IF NOT EXISTS tenant
(
    id          BIGINT(20) AUTO_INCREMENT PRIMARY KEY,
    uid         VARCHAR(255) NOT NULL COMMENT 'Tenant uid',
    zone        VARCHAR(20)  NOT NULL DEFAULT 'default' COMMENT 'Tenant zone',
    priority    INT          NOT NULL COMMENT 'Tenant priority',
    partition   VARCHAR(50)  NOT NULL COMMENT 'Partition this tenant belongs to',
    name        VARCHAR(100) NOT NULL COMMENT 'Tenant name',
    status      INT          NOT NULL COMMENT 'Tenant status',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the tenant was created at',
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'UTC time the tenant was updated at',
    last_active DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the tenant was last active',
    UNIQUE idx_uid (uid),
    INDEX idx_zone_partition (zone, partition)
) ENGINE = InnoDB COMMENT 'Tenant';

CREATE TABLE IF NOT EXISTS resourcequota
(
    id          BIGINT(20) AUTO_INCREMENT PRIMARY KEY,
    tenant_id   BIGINT(20) NOT NULL COMMENT 'Tenant id',
    type        INT        NOT NULL COMMENT 'Task type, crontask, delaytask or usertask',
    cpu         INT        NULL COMMENT 'CPU quota in millisecond',
    memory      INT        NULL COMMENT 'Memory quota in MB',
    storage     INT        NULL COMMENT 'Storage quota in MB',
    gpu         INT        NULL COMMENT 'GPU quota',
    concurrency INT        NULL COMMENT 'Concurrency quota',
    custom      INT        NULL COMMENT 'Custom workload figure',
    peak        FLOAT      NULL COMMENT 'Number of percent that actual or realtime resource usage can exceed quota',
    UNIQUE idx_tenant_id_type (tenant_id, type)
) ENGINE = InnoDB COMMENT 'Tenant resource quotas';

CREATE TABLE IF NOT EXISTS crontask
(
    id                BIGINT(20) AUTO_INCREMENT PRIMARY KEY,
    tenant_id         BIGINT(20)   NOT NULL COMMENT 'Tenant id',
    handler           VARCHAR(100) NOT NULL COMMENT 'Task handler',
    name              VARCHAR(100) NOT NULL COMMENT 'Task name',
    description       VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Task description',
    cron              VARCHAR(20)  NOT NULL COMMENT 'Cron expression, e.g. "0 0 18 28-31 * *"',
    config            JSON         NOT NULL COMMENT 'Task config',
    schedule_strategy INT          NOT NULL COMMENT 'Task schedule strategy',
    priority          INT          NOT NULL COMMENT 'Task priority',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the task was created at',
    updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'UTC time the task was updated at',
    next_tick         DATETIME     NOT NULL COMMENT 'Task next tick time in UTC',
    status            INT          NOT NULL COMMENT 'Task status',
    INDEX idx_tenant_id_next_tick (tenant_id, next_tick),
    INDEX idx_handler (handler)
) ENGINE = InnoDB COMMENT 'Cron tasks';

CREATE TABLE IF NOT EXISTS usertask
(
    id                BIGINT(20) AUTO_INCREMENT PRIMARY KEY,
    tenant_id         BIGINT(20)   NOT NULL COMMENT 'Tenant id',
    uid               VARCHAR(255) NOT NULL COMMENT 'Task uid',
    handler           VARCHAR(100) NOT NULL COMMENT 'Task handler',
    config            JSON         NOT NULL COMMENT 'Task config',
    schedule_strategy INT          NOT NULL COMMENT 'Task schedule strategy',
    priority          INT          NOT NULL COMMENT 'Task priority',
    progress          INT          NOT NULL DEFAULT 0 COMMENT 'Task progress, can be used as break point to continue processing',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the task was created at',
    updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'UTC time the task was updated at',
    status            INT          NOT NULL COMMENT 'Task status',
    UNIQUE idx_tenant_id_uid (tenant_id, uid),
    INDEX idx_handler (handler)
) ENGINE = InnoDB COMMENT 'User tasks';

CREATE TABLE IF NOT EXISTS delaytask
(
    id                BIGINT(20) AUTO_INCREMENT PRIMARY KEY,
    tenant_id         BIGINT(20)   NOT NULL COMMENT 'Tenant id',
    uid               VARCHAR(255) NOT NULL COMMENT 'Task uid',
    handler           VARCHAR(100) NOT NULL COMMENT 'Task handler',
    config            JSON         NOT NULL COMMENT 'Task config',
    schedule_strategy INT          NOT NULL COMMENT 'Task schedule strategy',
    priority          INT          NOT NULL COMMENT 'Task priority',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'UTC time the task was created at',
    updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'UTC time the task was updated at',
    time_to_run       DATETIME     NOT NULL COMMENT 'UTC time to run the delay task',
    status            INT          NOT NULL COMMENT 'Task status',
    UNIQUE idx_tenant_id_uid (tenant_id, uid),
    INDEX idx_tenant_id_time (tenant_id, time_to_run),
    INDEX idx_handler (handler)
) ENGINE = InnoDB COMMENT 'Delay tasks';

CREATE TABLE IF NOT EXISTS taskrun
(
    id            BIGINT(20) AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT(20)   NOT NULL COMMENT 'Tenant id',
    task_id       BIGINT(20)   NOT NULL COMMENT 'Task id',
    task_type     INT          NOT NULL COMMENT 'Task type, crontask, delaytask or usertask',
    schedule_type INT          NOT NULL COMMENT 'Task schedule type, maybe firstrun, retry or continue',
    status        INT          NOT NULL COMMENT 'Task run status',
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
    id  BIGINT(20) PRIMARY KEY COMMENT 'Task run id',
    log TEXT COMMENT 'Log'
) ENGINE = InnoDB COMMENT 'Task run log';
