# Chapter 16 — Design a Proximity Service (Yelp / Nearby Places)

## Q1: Compare Geohash, Google S2, and QuadTree for geospatial indexing at Yelp scale (100M+ places). What are the concrete trade-offs in query latency, write throughput, and cell neighbor complexity?

**Answer:** Geohash is a z-order curve with base32 encoding. Query: compute geohash prefix, scan that prefix's range in a KV store. Write: trivial single KV put. Neighbor lookup: must compute all 8 adjacent geohash cells manually — tedious but well-understood. Precision is fixed per prefix length. S2 uses a Hilbert curve on a cube-projected sphere, producing 64-bit cell IDs. Query: compute the covering (set of cells at a chosen level that covers the search circle). Write: single put. Neighbor lookup: adjacent cells have consecutive IDs due to Hilbert curve spatial locality, so range scans are efficient. S2 handles the spherical geometry correctly (geohash has distortion near poles). QuadTree: recursively subdivide space into four quadrants. Query: traverse the quadtree to find points in the target cell. Write: insert point, potentially split cells that exceed a capacity threshold. Neighbor query: walk up to parent then down to adjacent leaf. Quadtrees are dynamic (density-adaptive) but require more complex balancing and are harder to shard across servers. For 100M+ places, S2 is preferred by Google (it's used in Google Maps) because of spherical correctness, 64-bit efficiency, and excellent library support.

---

## Q2: A user searches for "coffee shops near me" at 14:30 on a Tuesday. How does the system combine location filtering, text relevance, and real-time business hours to return results in under 200 ms?

**Answer:** The system uses Elasticsearch with a geo-distance query filter: `{"bool": {"must": {"match": {"name": "coffee shop"}}, "filter": {"geo_distance": {"distance": "5km", "location": {"lat": X, "lon": Y}}}}}`. Business hours filtering is applied as a post-filter because it's not geo-indexed — each document has a `hours` field (e.g., `{"tuesday": [{"open": "07:00", "close": "22:00"}]}`). Elasticsearch returns matched docs, then the service layer filters by current time against business hours. To stay under 200 ms: (1) Elasticsearch geo queries with a bounded radius use the geohash cell prefix filter internally, making them fast. (2) The post-filter in the application layer is O(N) on the result set (typically < 100). (3) A CDN or Redis cache layers popular (lat/lng, query, time_of_day) combos. (4) Business hours can be precomputed into an "is_open" boolean field updated hourly via a batch job, moving the filter into the Elasticsearch query itself for faster performance.

---

## Q3: How would you shard a 100-billion-row places database across multiple database nodes while supporting efficient geo-range queries?

**Answer:** Shard by geohash prefix (e.g., 3-character prefix = ~150 km cells). Each shard owns one or more geohash prefixes. A query with a (lat, lng, radius) converts to a set of covering geohash prefixes at the appropriate level. The query is fanned out to the shards that own those prefixes, then results are merged in the application layer. This is how DynamoDB's geo library works. Challenges: (1) hot shards — Manhattan has far more places than rural Wyoming. Mitigate by splitting hot shards into smaller prefixes (longer geohash). (2) Cross-shard queries for the covering cells — the fan-out adds latency. Mitigate with parallel async queries. (3) Adding/removing shards requires rebalancing — use consistent hashing on the geohash prefix to minimize data movement.

---

## Q4: Design the proximity service's write path for a new restaurant opening. How does the system handle the update propagation across search indexes, caches, and secondary replicas within seconds?

**Answer:** The write path: (1) Restaurant owner submits details via API → the write service validates and writes to the primary metadata DB (PostgreSQL). (2) An event is published to a CDC stream (Debezium → Kafka). (3) Downstream consumers: (a) the search indexer consumes the event and upserts the document into Elasticsearch with the geo-point field; (b) the cache invalidator sends a cache invalidation message to Redis for any cached queries that might include this area (broadcast to nearby geohash prefixes); (c) the review indexer updates the review aggregation. (4) The API returns success to the client immediately after step 1 — the remaining propagation is async. Total end-to-end visibility: 1-3 seconds. For stronger consistency within seconds, Elasticsearch's refresh interval can be lowered to 1 second (at the cost of write throughput).

---

## Q5: A rideshare app needs to find all available drivers within 1 km. How do you design this for 10 million active drivers updating their location every 3 seconds? Compare pub/sub versus polling.

**Answer:** This is fundamentally different from static POI search because driver locations change every 3 seconds. Recommended approach: use a geospatial pub/sub system. Each rider subscribes to a geohash cell (e.g., precision level 7 ≈ 150 m). Drivers publish their location to the single cell they occupy. When a rider requests nearby drivers, the system subscribes to the rider's cell plus 8 neighbors. In Redis, use `GEOADD` to update driver location and `GEORADIUS` to query. Redis can handle ~100K GEOADD/s per node. For 10M drivers: shard by region across multiple Redis clusters. Alternative: use Apache Kafka with key by geohash cell — each consumer group receives all location updates for a specific area. Polling (e.g., driver writes to DB, rider queries DB) creates too much read/write amplification. The pub/sub approach reduces query latency because rider receives pushed updates rather than polling.

---

## Q6: How does the proximity service handle "edge cases" like a user standing exactly on a geohash boundary, or searching near the International Date Line or the North Pole?

**Answer:** For geohash boundaries: the query always expands the search to include all adjacent (8) neighbor cells. This 9-cell covering ensures that points near the boundary are included. For the International Date Line (±180° longitude): this is a discontinuity in most coordinate systems. Google S2 handles this natively with a Hilbert curve on a cube projection that wraps correctly. Geohash also handles it since longitude 180° wraps to -180° in the encoding, but you must compute neighbors that cross the date line correctly. For the North Pole (latitude 90°): geohash cells converge at the poles, making higher-precision geohash prefixes very small near the poles. S2 avoids polar distortion by projecting the sphere onto the faces of a cube. If your service doesn't operate near the poles (e.g., Uber, Yelp), geohash is fine. If global with polar regions (e.g., marine tracking), prefer S2.

---

## Q7: Design the "popular times" feature — showing how busy a place is at different hours. What's the data pipeline and storage format?

**Answer:** The pipeline: (1) Each check-in, search click, or explicit "I'm here" action produces a raw event with `{place_id, timestamp}`. (2) A stream processor (Flink/Dataflow) aggregates events into 15-minute buckets per place per day-of-week, running hourly. (3) The aggregated data `{place_id, day_of_week, hour_slot, visit_count}` is stored in a time-series friendly store (Bigtable or Cassandra) keyed by `(place_id, day_of_week)`. (4) The read path: when a user views a place page, the service fetches the 24-hour histogram for the current day-of-week and normalizes it to a percentage of peak (0-100%). Storage format: a compressed protobuf or array of 96 uint16 values (one per 15-minute slot). For "live busyness", compare the current slot's real-time count against the historical baseline.

---

## Q8: How would you design a recommendation system that biases search results toward places the user has previously visited or rated highly?

**Answer:** This requires a personalization layer that merges with the geo-text search. Approach: (1) Maintain a user embedding or preference vector (favorite cuisines, price range, rating threshold) in a user service. (2) When a search query arrives, the search service enriches the Elasticsearch query with a boosting function: `{"function_score": {"functions": [{"filter": {"term": {"place_id": <visited_place_id>}}, "weight": 1.5}]}}`. (3) For collaborative filtering, store a user-place interaction matrix in a graph DB or sparse matrix store. Use approximate nearest neighbor (ANN) search on place embeddings to find similar places the user liked. (4) Merge results: 70% weight on geo-text relevance, 30% on personalization score. The personalization model is trained offline (nightly batch), and the embeddings are loaded into a sidecar cache on each search node for low-latency lookup.

---

## Q9: The proximity service receives a sudden spike in traffic (e.g., New Year's Eve). How does the system scale to handle 100x normal load?

**Answer:** Multi-layered approach: (1) Read path: most queries hit the CDN edge cache for popular area searches. Geo queries that cannot be cached (the user is in a unique location) still require origin hits. (2) Elasticsearch cluster auto-scales by adding search replicas (horizontal scaling for read throughput). (3) The API gateway applies load shedding: non-critical queries (reviews, photos) are degraded or queued; critical queries (nearby results) get priority. (4) The metadata DB uses read replicas to absorb the increased query load. (5) Caching: precompute and cache queries for known high-density event areas (e.g., Times Square on NYE) in a local Redis cluster. (6) If the system is still overwhelmed, return cached results with a "last updated X minutes ago" notice rather than failing.

---

## Q10: How would you design a "real-time" leaderboard that ranks places by check-in velocity (most check-ins in the last hour)?

**Answer:** Use a sliding window counter per place in Redis: `ZADD velocity:current_hour <timestamp> <place_id>` with an expiry. Maintain a sorted set `ZINCRBY velocity:leaderboard 1 <place_id>` on each check-in event. Every minute, a cron job expires old entries (outside the 1-hour window) by comparing against a cursor of the oldest allowed event. The leaderboard is simply `ZREVRANGE velocity:leaderboard 0 99`. For high throughput (millions of check-ins/min): windowed aggregation in a stream processor (Flink), writing per-minute counts to Redis: `ZINCRBY velocity:leaderboard <count> <place_id>`. The Flink job handles windowed deduplication, and Redis handles the real-time ranking query. Use a separate Redis instance for the leaderboard to avoid impacting the geo-indexing Redis.
