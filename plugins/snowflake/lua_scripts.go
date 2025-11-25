package snowflake

// Lua scripts for snowflake ID generator Redis operations
// All scripts are executed atomically by Redis

// LuaScriptIncrWithReset atomically increments a counter and resets it if exceeds max
// KEYS[1]: counter key
// ARGV[1]: max value (totalWorkerIDs)
// Returns: the counter value (1 to max)
const LuaScriptIncrWithReset = `
local counter = redis.call('INCR', KEYS[1])
if counter > tonumber(ARGV[1]) then
    redis.call('SET', KEYS[1], '1')
    return 1
end
return counter
`

// LuaScriptHeartbeat atomically verifies instanceID and updates heartbeat
// KEYS[1]: worker key
// ARGV[1]: new worker info JSON
// ARGV[2]: expected instanceID
// ARGV[3]: TTL in seconds
// Returns: 1=success, 0=instanceID mismatch, -1=key not exist, -2=invalid format
const LuaScriptHeartbeat = `
local current = redis.call('GET', KEYS[1])
if not current then
    return -1
end
local instanceId = string.match(current, '"instance_id":"([^"]+)"')
if not instanceId then
    return -2
end
if instanceId ~= ARGV[2] then
    return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[3])
return 1
`
