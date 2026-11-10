package redis

const TokenBucketScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local refill_rate = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_updated')
local tokens = capacity
local last_updated = now

if bucket[1] and bucket[2] then
	tokens = tonumber(bucket[1])
	last_updated = tonumber(bucket[2])
end

local delta = math.max(0, now - last_updated)
local tokens_to_add = delta * refill_rate
tokens = math.min(capacity, tokens + tokens_to_add)

if tokens >= requested then
	tokens = tokens - requested
	redis.call('HSET', key, 'tokens', tokens, 'last_updated', now)

	local time_to_fill = math.ceil(capacity / refill_rate) + 2
	redis.call('EXPIRE', key, time_to_fill)

	return {1, tokens}
else
	local deficit = requested - tokens
	local retry_after = math.ceil(deficit / refill_rate)

	redis.call('HSET', key, 'tokens', tokens, 'last_updated', last_updated)

	return {0, tokens, retry_after}
end
`
const LeakyBucketScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local leak_rate = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

-- 1. Fetch existing water level and last leak time
local bucket = redis.call('HMGET', key, 'water_level', 'last_leak_time')
local water_level = 0
local last_leak_time = now

if bucket[1] and bucket[2] then
	water_level = tonumber(bucket[1])
	last_leak_time = tonumber(bucket[2])
end

-- 2. Lazy Leak Math: Drain water based on time elapsed
local delta = math.max(0, now - last_leak_time)
local leaked = delta * leak_rate
water_level = math.max(0, water_level - leaked)

-- 3. Check if new request fits in the bucket
if water_level + requested <= capacity then
	water_level = water_level + requested
	redis.call('HSET', key, 'water_level', water_level, 'last_leak_time', now)

	-- Set TTL so idle keys auto-expire once fully drained
	local time_to_empty = math.ceil(water_level / leak_rate) + 2
	redis.call('EXPIRE', key, time_to_empty)

	return {1, capacity - water_level}
else
	-- Calculate wait time (in seconds) until enough space leaks out
	local excess = (water_level + requested) - capacity
	local retry_after = math.ceil(excess / leak_rate)

	redis.call('HSET', key, 'water_level', water_level, 'last_leak_time', now)

	return {0, capacity - water_level, retry_after}
end
`
