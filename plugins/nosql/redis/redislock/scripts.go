package redislock

// 获取锁的 Lua 脚本
const lockScript = `
local key = KEYS[1]
local value = ARGV[1]
local expiration = ARGV[2]
local reentrantKey = KEYS[1] .. ":reentrant:" .. ARGV[1]

-- 检查是否是可重入锁
local count = redis.call("GET", reentrantKey)
if count then
    -- 如果是同一个持有者，增加计数器
    redis.call("INCR", reentrantKey)
    redis.call("PEXPIRE", key, expiration)
    redis.call("PEXPIRE", reentrantKey, expiration)
    return "OK"
end

-- 如果锁不存在，获取锁
if redis.call("EXISTS", key) == 0 then
    redis.call("SET", key, value, "PX", expiration, "NX")
    redis.call("SET", reentrantKey, "1", "PX", expiration)
    return "OK"
end

return "LOCKED"`

// 释放锁的 Lua 脚本
const unlockScript = `
local key = KEYS[1]
local value = ARGV[1]
local reentrantKey = key .. ":reentrant:" .. value

-- 检查是否是当前持有者
if redis.call("GET", key) ~= value then
    if redis.call("EXISTS", key) == 0 then
        return 0 -- 锁不存在
    end
    return -1 -- 锁存在但不是当前持有者
end

-- 获取重入计数
local count = redis.call("GET", reentrantKey)
if not count then
    -- 如果重入计数不存在，直接删除锁
    redis.call("DEL", key)
    return 1
end

-- 减少重入计数
local newCount = redis.call("DECR", reentrantKey)
if newCount == 0 then
    -- 如果计数为 0，删除锁和重入计数
    redis.call("DEL", key, reentrantKey)
else
    -- 否则更新过期时间
    local expiration = redis.call("PTTL", key)
    if expiration > 0 then
        redis.call("PEXPIRE", reentrantKey, expiration)
    end
end
return 1`

// 续期锁的 Lua 脚本
const renewScript = `
local key = KEYS[1]
local value = ARGV[1]
local expiration = ARGV[2]
local reentrantKey = key .. ":reentrant:" .. value

-- 检查锁是否存在且是否为当前持有者
local currentValue = redis.call("GET", key)
if currentValue == value then
    -- 如果是当前持有者，则续期
    if redis.call("PEXPIRE", key, expiration) == 1 then
        -- 同时续期重入计数器
        if redis.call("EXISTS", reentrantKey) == 1 then
            redis.call("PEXPIRE", reentrantKey, expiration)
        end
        return 1
    end
    return -2 -- PEXPIRE 失败，锁不存在
end

-- 锁不存在
if currentValue == false then
    return -1
end

-- 锁存在但不是当前持有者
return 0`
