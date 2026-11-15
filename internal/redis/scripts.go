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

const FixedWindowScript = `
local key = KEYS[1]
local window_size = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- Determine current window bucket timestamp
local current_window = math.floor(now / window_size)
local window_key = key .. ":" .. current_window

local current_count = tonumber(redis.call('GET', window_key) or "0")

if current_count + requested > limit then
	local window_end = (current_window + 1) * window_size
	local retry_after_sec = window_end - now
	if retry_after_sec < 1 then
		retry_after_sec = 1
	end
	local remaining = limit - current_count
	if remaining < 0 then remaining = 0 end
	return {0, remaining, retry_after_sec}
else
	local new_count = redis.call('INCRBY', window_key, requested)
	if current_count == 0 then
		-- Expire key after twice the window size to clean up old windows safely
		redis.call('EXPIRE', window_key, window_size * 2)
	end
	local remaining = limit - new_count
	if remaining < 0 then remaining = 0 end
	return {1, remaining, 0}
end
`

const SlidingWindowLogScript = `
local key = KEYS[1]
local window_size = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local member_id = ARGV[4]

-- Fetch current time directly from Redis (fully compatible with Miniredis FastForward)
local time_arr = redis.call('TIME')
local now = tonumber(time_arr[1])

local clear_before = now - window_size

-- 1. Remove expired timestamps outside the rolling window
redis.call('ZREMRANGEBYSCORE', key, '-inf', clear_before)

-- 2. Count current active requests in the sliding window
local current_count = tonumber(redis.call('ZCARD', key))

if current_count + requested > limit then
    -- Find the oldest request score to compute accurate retry-after time
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after_sec = window_size
    if #oldest >= 2 then
        local oldest_score = tonumber(oldest[2])
        retry_after_sec = (oldest_score + window_size) - now
        if retry_after_sec < 1 then
            retry_after_sec = 1
        end
    end
    local remaining = limit - current_count
    if remaining < 0 then remaining = 0 end
    return {0, remaining, math.ceil(retry_after_sec)}
else
    -- Add the new request timestamp to the sorted set
    redis.call('ZADD', key, now, member_id)
    -- Refresh expiration on the sorted set to prevent memory leaks
    redis.call('EXPIRE', key, math.ceil(window_size * 2))

    local new_count = current_count + requested
    local remaining = limit - new_count
    if remaining < 0 then remaining = 0 end
    return {1, remaining, 0}
end
`
