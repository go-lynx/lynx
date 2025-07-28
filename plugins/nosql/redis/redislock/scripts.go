package redislock

// 获取锁的 Lua 脚本
// KEYS[1]: 锁的键名
// ARGV[1]: 锁的值（用于识别持有者）
// ARGV[2]: 锁的过期时间（毫秒）
// 返回值: "OK" - 获取成功, "LOCKED" - 锁已被其他持有者持有
const lockScript = `
-- 检查锁是否存在
if redis.call("EXISTS", KEYS[1]) == 0 then
    -- 锁不存在，尝试设置锁
    local result = redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX")
    if result then
        return "OK"
    else
        return "LOCKED"
    end
end

-- 锁已存在，检查是否是当前持有者（支持可重入）
if redis.call("GET", KEYS[1]) == ARGV[1] then
    -- 是当前持有者，续期锁
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    return "OK"
end

-- 锁被其他持有者持有
return "LOCKED"`

// 释放锁的 Lua 脚本
// KEYS[1]: 锁的键名
// ARGV[1]: 锁的值（用于识别持有者）
// 返回值: 1 - 成功释放, 0 - 锁不存在, -1 - 锁存在但不是当前持有者
const unlockScript = `
-- 检查锁是否存在且是否为当前持有者
if redis.call("GET", KEYS[1]) == ARGV[1] then
    -- 是当前持有者，删除锁
    redis.call("DEL", KEYS[1])
    return 1
end

-- 检查锁是否存在
if redis.call("EXISTS", KEYS[1]) == 0 then
    -- 锁不存在
    return 0
end

-- 锁存在但不是当前持有者
return -1`

// 续期锁的 Lua 脚本
// KEYS[1]: 锁的键名
// ARGV[1]: 锁的值（用于识别持有者）
// ARGV[2]: 新的过期时间（毫秒）
// 返回值: 1 - 续期成功, 0 - 锁存在但不是当前持有者, -1 - 锁不存在, -2 - 续期失败
const renewScript = `
-- 检查锁是否存在且是否为当前持有者
local value = redis.call("GET", KEYS[1])
if value == ARGV[1] then
    -- 是当前持有者，尝试续期
    if redis.call("PEXPIRE", KEYS[1], ARGV[2]) == 1 then
        return 1
    end
    -- PEXPIRE 失败，说明锁已经不存在
    return -2
end

-- 锁不存在
if value == false then
    return -1
end

-- 锁存在但不是当前持有者
return 0`
