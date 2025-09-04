package redislock

import (
	"github.com/redis/go-redis/v9"
)

// This file contains three Lua scripts (acquire, release, renew), encapsulated as go-redis Script for EVALSHA reuse.
// Important design conventions (must be strictly consistent with Go side):
//   - lockLua: Return value > 0 indicates reentry count of current holder; 0 indicates occupied by other holder.
//   - unlockLua: Return 2 for partial release (count > 0), 1 for complete release, 0 for non-existent, -1 for non-holder.
//     Note: When partially releasing, expiration time of both keys is only refreshed when passed TTL (milliseconds) > 0;
//     For unified semantics, Go side defaults to passing 0 (no TTL refresh) for "partial release" in Unlock/UnlockByValue.
//   - renewLua: Return 1 for successful renewal; 0 for non-holder; -1 for non-existent; -2 for renewal failure.
//
// Additionally: All scripts use KEYS[1]=owner key, KEYS[2]=count key and share the same hashtag under Redis Cluster,
// to ensure atomic operations within the same slot.
// Lua script for acquiring lock (with reentry count)
// KEYS[1]: owner key (stores holder identifier)
// KEYS[2]: count key (stores reentry count)
// ARGV[1]: owner value (lock value, used to identify holder)
// ARGV[2]: expiration time (milliseconds)
var lockLua = `
local owner = redis.call("GET", KEYS[1])
if not owner then
    -- No owner, try to acquire
    local ok = redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX")
    if ok then
        redis.call("SET", KEYS[2], 1, "PX", ARGV[2])
        return 1
    else
        return 0
    end
end

if owner == ARGV[1] then
    -- Reentrant: increment count and renew both keys
    local cnt = redis.call("INCR", KEYS[2])
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    redis.call("PEXPIRE", KEYS[2], ARGV[2])
    return cnt
end

-- Occupied by other holder
return 0`

// Lua script for releasing lock (with reentry count)
// KEYS[1]: owner key
// KEYS[2]: count key
// ARGV[1]: owner value
// ARGV[2]: TTL refresh expiration time (milliseconds), used for PEXPIRE when partially releasing
// Return: 2 partial release (count decremented still > 0); 1 complete release (delete keys); 0 non-existent; -1 non-holder
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
    -- Still held: optionally refresh TTL and return partial release (only refresh when passed TTL > 0)
    local ttl = tonumber(ARGV[2])
    if ttl and ttl > 0 then
        redis.call("PEXPIRE", KEYS[1], ttl)
        redis.call("PEXPIRE", KEYS[2], ttl)
    end
    return 2
end

-- Count <= 0, complete release
redis.call("DEL", KEYS[1])
redis.call("DEL", KEYS[2])
return 1`

// Lua script for renewing lock (with reentry count)
// KEYS[1]: owner key
// KEYS[2]: count key
// ARGV[1]: owner value
// ARGV[2]: new expiration time (milliseconds)
// Return: 1 successful renewal; 0 non-holder; -1 non-existent; -2 renewal failure
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

// go-redis Script objects, using EVALSHA cache
// Go side usage mapping:
//
//	lockScript:   acquire lock
//	unlockScript: release lock
//	renewScript:  renew lock
var (
	lockScript   = redis.NewScript(lockLua)
	unlockScript = redis.NewScript(unlockLua)
	renewScript  = redis.NewScript(renewLua)
)
