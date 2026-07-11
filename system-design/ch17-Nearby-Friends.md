# Chapter 17 — Design Nearby Friends

## Q1: A feature like Snapchat's Snap Map or Find My shows a user's friends within a certain radius on a map. How do you balance location update frequency against battery life and bandwidth?

**Answer:** Use adaptive location reporting based on user state. When the app is in the foreground and the user is actively viewing the map, poll GPS at high frequency (every 5-10 seconds). When in the background or the app is closed, use geofence-based updates — a significant location change (>500 meters) or periodic updates (every 15-30 minutes). iOS and Android provide "significant location change" APIs that are battery-efficient (wake the device only on cell tower handoffs). Further optimization: if the user hasn't moved in 10 minutes, switch to a "stationary" state and reduce polling to once every 5 minutes.

---

## Q2: Design the server-side architecture for a "Nearby Friends" feature supporting 100 million daily active users. How do you match friends within a geospatial radius without a full table scan?

**Answer:** Shard by friend group or region. Two main approaches: (1) **User-centric sharding**: each user's friends list maps to a small set of user IDs. Instead of geo-indexing all users globally, only index the locations of a user's friends. When user A opens the map, fetch the last known locations of all of A's friends (a bounded set, typically < 1000) from a Redis cache keyed by `user_id`. Then filter by distance in the application layer. This avoids a global geo-index entirely. (2) **Geo-centric sharding**: for the open-world approach (Snap Map where you can see public stories near you), use a geohash-based index in Redis. For each user, `GEOADD friends:<user_id> <lng> <lat> <friend_id>`. When querying, `GEORADIUS friends:<user_id> <lng> <lat> <radius>`. The `friends:<user_id>` sorted set is stored on a Redis node determined by consistent hashing on `user_id`.

---

## Q3: How do you handle privacy in a Nearby Friends system? Users should be able to hide from specific people or enable "ghost mode" at any time.

**Answer:** Implement a permission filter layer. When a user updates their location, it's written to a location cache with `{user_id, lat, lng, timestamp, visibility}`. When friend B requests to see friend A's location: (1) Check if A has B blocked (block list lookup, O(1)). (2) Check if A has "ghost mode" enabled (boolean flag on A's profile). (3) Check if A is sharing with B specifically or with a custom list. This filtering is done in the read path, not the write path, because a user's privacy settings can change between location updates. The permission check must be fast (<5 ms per friend) — cache the user's privacy settings in Redis. For batch requests (fetching locations of 50 friends), issue 50 parallel Redis `MGET` calls with a 50-ms deadline.

---

## Q4: A user has 5,000 friends. Fetching all their locations on map open would be expensive. How does Snap Map handle this at scale?

**Answer:** Snap Map uses a combination of techniques: (1) The client only displays friends visible within the current map viewport. The server only returns friends whose last known location falls within the viewport bounds + some margin. (2) The client sends the current viewport bounds (SW/NE lat/lng) along with the request. The server performs a bounding-box filter on the cached friend locations. (3) Location data is stored in a spatial-temporal key-value store keyed by `user_id` with a TTL. The server iterates through the user's friend list (pulled from a social graph service) and checks each friend's last location against the viewport. (4) This is an O(N_friends) operation, but for 5,000 friends, a bounding-box check is a few float comparisons — it completes in <50 ms if the data is in Redis. (5) Paginate — return the closest 100 friends first, then lazy-load others as the user pans.

---

## Q5: Design the WebSocket-based real-time push mechanism. When friend B moves, how does friend A's map update within seconds?

**Answer:** When user A opens the map, the mobile client establishes a persistent WebSocket connection to a notification server. The server subscribes A to location updates from A's friends (essentially a pub/sub channel per friend group). When B's location changes: (1) B's client sends the new location to the API. (2) The API writes to the location cache. (3) An event is published to a Redis pub/sub or Kafka topic for B's user ID. (4) A fanout service reads B's friend list from the social graph (cached) and routes the location event to the notification servers that hold WebSocket connections for each of B's friends. (5) Those notification servers push the location update to A's client. This avoids A's client polling. To scale fan-out: use a Redis Cluster fronting the notification servers. Each notification server maintains an in-memory map of `user_id → WebSocket connection`. When a friend moves, the notification router uses a consistent-hash ring to find which notification server holds that friend's WebSocket.

---

## Q6: How do you handle the "last seen" timestamp for location data? What's the staleness threshold before showing "location unavailable"?

**Answer:** Location data has an associated timestamp and a TTL in the cache (e.g., 2 hours). When the client requests friend locations, the server returns `{lat, lng, timestamp}` for each friend. The client decides how to display it based on staleness: <1 minute → bright dot; 1-15 minutes → faded dot; >15 minutes → show "X minutes ago" label; >2 hours → "location unavailable". The server-side TTL ensures stale data doesn't accumulate. For the "live" indicator (green dot), the server checks if the timestamp is within the last 2 minutes. Configurable thresholds are useful — during a hurricane or festival, you might extend the freshness threshold to reduce battery drain from frequent polling.

---

## Q7: How would you test chaotic scenarios — what happens when 100,000 people are in the same concert venue and all refreshing their map simultaneously?

**Answer:** This creates a thundering herd on the location read path. Mitigations: (1) Cache the location data aggressively — since many users are querying for the same set of nearby friends. Use a local cache (e.g., in-process LRU) on each API server with a 10-second TTL. (2) Rate-limit the client — the mobile app should back off if the server returns 429 responses, using exponential backoff. (3) Use a "viewports" cache — if two clients query the same geohash area within seconds, serve the same cached response. (4) Degrade gracefully — if the map service is overloaded, show cached locations from the last successful poll with a "last updated X ago" label rather than showing nothing.

---

## Q8: How do you design the location sharing invitation flow — "Share my location for 1 hour" — with temporary, revokable access?

**Answer:** Temporary sharing uses expiring tokens with access control entries (ACE). When user A shares their location with user B for 1 hour: (1) Create an ACE `{grantor: A, grantee: B, permission: READ_LOCATION, expires_at: now + 1h}` in a fast KV store keyed by `(grantee_id, grantor_id)`. (2) Set a TTL on the ACE equal to the sharing duration. (3) On each location read, the server checks the ACE — if expired, deny access. (4) Revocation is immediate — delete the ACE from the KV store before its TTL expires. (5) For recurring sharing (every day from 9-5), use a cron-style eval that re-creates the ACE on schedule. The ACE check must be in the critical path (read path) and uses a cache with sub-millisecond latency. Write the ACE to both the primary DB (for durability) and Redis (for fast reads).

---

## Q9: How does Nearby Friends handle cross-platform and cross-region latency when users are distributed globally?

**Answer:** Use a multi-region deployment with active-active geo-replicated services. Location writes go to the nearest regional endpoint (DNS-based routing). Location data is replicated across regions asynchronously (e.g., via Kafka MirrorMaker or Cassandra multi-region replication). When user A in Europe looks for friend B in Asia: (1) A's request hits the EU region. (2) The EU region has asynchronously replicated B's location (typically <5 second lag). (3) The response shows B's location from the replicated data. Cross-region read latency is negligible because the data is cached in the local region. For the "live" indicator (green dot), the freshness is defined relative to the original timestamp, not the region clock. Cross-region latency of 100-200 ms for the write path is acceptable for a feature that doesn't require sub-second consistency.

---

## Q10: A friend is traveling on a highway at 120 km/h. How does the system handle the burst of location updates without overwhelming the pipeline?

**Answer:** Windowing and deduplication. At 120 km/h (33 m/s), a location update every second would flood the system. Strategies: (1) Client-side throttling — the mobile app enforces a minimum interval between uploads (e.g., 5 seconds) regardless of GPS changes. (2) Server-side dedup — if the new location is within 50 meters of the last reported location (at highway speed, 5 seconds × 33 m/s = 165 meters), the update is still significant. The server can batch updates: receive every update but only publish to subscribers every 10 seconds. (3) Adaptive resolution — at high speed, location accuracy requirements are lower (you don't need centimeter precision for a map dot). The server can quantize the coordinates to fewer decimal places (e.g., 3 decimal places ≈ 110 meter precision vs. 5 decimal places ≈ 1 meter). This increases the effective cache hit rate and reduces the cost of geo-distance computations.
