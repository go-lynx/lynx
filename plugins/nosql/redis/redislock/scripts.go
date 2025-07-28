package redislock

// 获取锁的 Lua 脚本
const lockScript = `
if redis.call("EXISTS", KEYS[1]) == 0 then
    -- 设置锁的值和过期时间
    redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX")
    return "OK"
end

-- 如果锁已存在，检查是否是当前持有者
-- 这里可以扩展实现可重入锁
if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    return "OK"
end

return "LOCKED"`

// 释放锁的 Lua 脚本
const unlockScript = `
-- 检查锁是否存在且是否为当前持有者
if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("DEL", KEYS[1])
    return 1
end

-- 检查锁是否存在
if redis.call("EXISTS", KEYS[1]) == 0 then
    return 0
end

-- 锁存在但不是当前持有者
return -1`

// 续期锁的 Lua 脚本
const renewScript = `
-- 检查锁是否存在且是否为当前持有者
local value = redis.call("GET", KEYS[1])
if value == ARGV[1] then
    -- 如果是当前持有者，则续期
    if redis.call("PEXPIRE", KEYS[1], ARGV[2]) == 1 then
        return 1
    end
    -- 如果 PEXPIRE 失败，说明锁已经不存在
    return -2
end

-- 锁不存在
if value == false then
    return -1
end

-- 锁存在但不是当前持有者
return 0`
