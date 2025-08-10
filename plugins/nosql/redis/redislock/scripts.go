package redislock

import (
	"github.com/redis/go-redis/v9"
)

// 本文件包含三段 Lua 脚本（获取、释放、续期），并以 go-redis Script 封装供 EVALSHA 复用。
// 重要设计约定（需与 Go 侧严格一致）：
//   - lockLua: 返回值>0 表示当前持有者的重入计数；0 表示被其他持有者占用。
//   - unlockLua: 返回 2 表示部分释放(计数>0)，1 表示完全释放，0 不存在，-1 非持有者。
//     注意：当部分释放时，仅当传入的 TTL(毫秒) > 0 才会刷新两个键的过期时间；
//     为了统一语义，Go 侧对 Unlock/UnlockByValue 的“部分释放”默认传 0（不刷新 TTL）。
//   - renewLua: 返回 1 表示续期成功；0 非持有者；-1 不存在；-2 续期失败。
//
// 另外：所有脚本使用 KEYS[1]=owner 键、KEYS[2]=count 键 且在 Redis Cluster 下共享相同 hashtag，
// 以确保在同一 slot 内执行原子操作。
// 获取锁的 Lua 脚本（带重入计数）
// KEYS[1]: owner 键（保存持有者标识）
// KEYS[2]: count 键（保存重入计数）
// ARGV[1]: owner 值（锁值，用于识别持有者）
// ARGV[2]: 过期时间（毫秒）
var lockLua = `
local owner = redis.call("GET", KEYS[1])
if not owner then
    -- 无 owner，尝试占有
    local ok = redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX")
    if ok then
        redis.call("SET", KEYS[2], 1, "PX", ARGV[2])
        return 1
    else
        return 0
    end
end

if owner == ARGV[1] then
    -- 可重入：计数 +1 并续期两个键
    local cnt = redis.call("INCR", KEYS[2])
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    redis.call("PEXPIRE", KEYS[2], ARGV[2])
    return cnt
end

-- 被其他持有者占用
return 0`

// 释放锁的 Lua 脚本（带重入计数）
// KEYS[1]: owner 键
// KEYS[2]: count 键
// ARGV[1]: owner 值
// ARGV[2]: 刷新 TTL 的过期时间（毫秒），当部分释放时用于 PEXPIRE
// 返回: 2 部分释放(计数减一仍>0)；1 完全释放(删除键)；0 不存在；-1 非持有者
var unlockLua = `
local owner = redis.call("GET", KEYS[1])
if not owner then
    return 0
end
if owner ~= ARGV[1] then
    return -1
end

local cnt = redis.call("DECR", KEYS[2])
if cnt and cnt > 0 then
    -- 仍持有：可选择刷新 TTL 并返回部分释放（仅当传入 TTL>0 时刷新）
    local ttl = tonumber(ARGV[2])
    if ttl and ttl > 0 then
        redis.call("PEXPIRE", KEYS[1], ttl)
        redis.call("PEXPIRE", KEYS[2], ttl)
    end
    return 2
end

-- 计数<=0，完全释放
redis.call("DEL", KEYS[1])
redis.call("DEL", KEYS[2])
return 1`

// 续期锁的 Lua 脚本（带重入计数）
// KEYS[1]: owner 键
// KEYS[2]: count 键
// ARGV[1]: owner 值
// ARGV[2]: 新的过期时间（毫秒）
// 返回: 1 续期成功；0 非持有者；-1 不存在；-2 续期失败
var renewLua = `
local owner = redis.call("GET", KEYS[1])
if not owner then
    return -1
end
if owner ~= ARGV[1] then
    return 0
end

local ok1 = redis.call("PEXPIRE", KEYS[1], ARGV[2])
local ok2 = redis.call("PEXPIRE", KEYS[2], ARGV[2])
if ok1 == 1 and ok2 == 1 then
    return 1
end
return -2`

// go-redis Script 对象，使用 EVALSHA 缓存
// Go 侧使用的映射：
//
//	lockScript:   获取锁
//	unlockScript: 释放锁
//	renewScript:  续期锁
var (
	lockScript   = redis.NewScript(lockLua)
	unlockScript = redis.NewScript(unlockLua)
	renewScript  = redis.NewScript(renewLua)
)
