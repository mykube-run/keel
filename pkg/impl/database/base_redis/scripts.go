package base_redis

import "github.com/redis/go-redis/v9"

const common = `local log_level = redis.LOG_WARNING

-- returns tenant buckets key
local function tenant_buckets_key(tenant_id)
    return string.format('task-buckets:{%s}', tenant_id)
end

-- returns bucket key
local function bucket_key(tenant_id, bucket_ts)
    return string.format('task-bucket:{%s}:%s', tenant_id, bucket_ts)
end

-- returns task key
local function task_key(tenant_id, uid)
    return string.format('task:{%s}:%s', tenant_id, uid)
end

-- get task by uid, returns encoded JSON string
local function get_task(tenant_id, uid)
    local key = task_key(tenant_id, uid)
    redis.log(log_level, string.format('[GetTask] get task: %s', key))
    return redis.call('GET', key)
end
`

var updateTaskStatus = redis.NewScript(common + `
-- update specified task status, uid
local function update_task_status(tenant_id, uid, status)
    local task = cjson.decode(get_task(tenant_id, uid))
    local key = task_key(tenant_id, uid)
    task['status'] = status
    redis.call('SET', key, cjson.encode(task), 'KEEPTTL')
    redis.log(log_level, string.format('[UpdateTaskStatus] updated status of %s to %s', key, status))

    local bucket_ts = task['__bucket_ts__']
    local bucket = bucket_key(tenant_id, bucket_ts)

    -- task status is not Pending, remove it from task bucket
    if status ~= 'Pending'
    then
        redis.call('ZREM', bucket, uid)
        redis.log(log_level, string.format('[UpdateTaskStatus] removed %s from bucket %s', uid, bucket))
    else
        local buckets = tenant_buckets_key(tenant_id)
        redis.call('ZADD', bucket, tonumber(task_ts), uid)
        redis.call('ZADD', buckets, tonumber(bucket_ts), bucket_ts)
        redis.log(log_level, string.format('[UpdateTaskStatus] added %s in bucket %s', uid, bucket))
    end
end

-- update specified tasks status, uids_str is JSON array string
local function update_tasks_status(tenant_id, uids_str, status)
	redis.log(log_level, string.format('[UpdateTaskStatus] uids: %s, status: %s', uids_str, status))
	local uids = cjson.decode(uids_str)
	local count = 0
	for idx, value in ipairs(uids) do
		update_task_status(tenant_id, value, status)
		count = count + 1
	end
	return count
end

return update_tasks_status(KEYS[1], ARGV[1], ARGV[2])
`)

var findPendingTasks = redis.NewScript(common + `
-- find at most limit tasks by tenant id
local function find_pending_tasks(tenant_id, limit)
    local buckets_key = tenant_buckets_key(tenant_id)
    local buckets = redis.call('ZRANGEBYSCORE', buckets_key, 0, '+inf', 'LIMIT', 0, limit)

    local bucket_range_limit = limit
    local tasks = {}
    for idx, value in ipairs(buckets) do
        local bucket = bucket_key(tenant_id, value)
        local bucket_tasks = redis.call('ZRANGEBYSCORE', bucket, '0', '+inf', 'LIMIT', 0, bucket_range_limit)
        bucket_range_limit = bucket_range_limit - #bucket_tasks

        if (bucket_range_limit <= 0)
        then
            return tasks
        end

        for idx2, value2 in ipairs(bucket_tasks) do
            table.insert(tasks, get_task(tenant_id, value2))
        end

        local msg = string.format('[FindPendingTasks] specified tenant id: %s, limit: %s, bucket: %s, tasks fetched from bucket: %d, total number of tasks: %d', tenant_id, limit, bucket, #bucket_tasks, #tasks)
        redis.log(log_level, msg)
    end

    return tasks
end

return find_pending_tasks(KEYS[1], ARGV[1])
`)

var createTask = redis.NewScript(common + `
-- create a new task
local function create_task(tenant_id, uid, task_str, ttl)
    local task = cjson.decode(task_str)
    local task_ts = task['__task_ts__']
    local bucket_ts = task['__bucket_ts__']

    local key = task_key(tenant_id, uid)
    redis.call('SET', key, task_str, 'EX', ttl)

    local bucket = bucket_key(tenant_id, bucket_ts)
    redis.call('ZADD', bucket, tonumber(task_ts), uid)

    local buckets = tenant_buckets_key(tenant_id)
    redis.call('ZADD', buckets, tonumber(bucket_ts), bucket_ts)

    redis.log(log_level, string.format('[CreateTask] created task: %s, bucket timestamp: %s', key, bucket_ts))
    return key
end

return create_task(KEYS[1], ARGV[1], ARGV[2], ARGV[3])
`)

var countTenantPendingTasks = redis.NewScript(common + `
-- count specified tenant's pending tasks
local function count_tenant_pending_tasks(tenant_id)
    local buckets_key = tenant_buckets_key(tenant_id)
    local buckets = redis.call('ZRANGEBYSCORE', buckets_key, '0', '+inf')
    local count = 0

    for idx, value in ipairs(buckets) do
        local bucket = bucket_key(tenant_id, value)
        local bucket_count = redis.call('ZCARD', bucket)
        count = count + bucket_count
    end

    redis.log(log_level, string.format('[CountTenantPendingTasks] specified tenant id: %s, buckets: %d, count: %d', tenant_id, #buckets, count))
    return count
end

return count_tenant_pending_tasks(KEYS[1])
`)

var getTask = redis.NewScript(common + `
return get_task(KEYS[1], ARGV[1])
`)

var getTaskStatus = redis.NewScript(common + `
local function get_task_status(tenant_id, uid)
	local task = cjson.decode(get_task(tenant_id, uid))
	return task['status']
end

return get_task_status(KEYS[1], ARGV[1])
`)
