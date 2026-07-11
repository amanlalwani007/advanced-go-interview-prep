# Chapter 15 — Design Google Drive

## Q1: Explain how Google Drive handles delta sync for efficient revision storage. How does it decide which blocks have changed without transferring the entire file?

**Answer:** Google Drive uses a chunk-based delta sync algorithm similar to rsync. The file is split into fixed-size blocks (e.g., 4 MB). On each sync, the client computes hash checksums (SHA-256) for all blocks and sends them to the server. The server compares these hashes against the last committed revision's block index. Only blocks with mismatched hashes are uploaded. This avoids re-uploading the entire file. For large files with small changes (e.g., a 1 GB presentation with one slide changed), only the modified ~4 MB blocks are transferred. The server then assembles the new revision as a manifest pointing to the same (unmodified) block references plus the newly uploaded blocks.

---

## Q2: Design the architecture for handling 10,000+ simultaneous file uploads per second. Where are the bottlenecks, and how do you shard the metadata store?

**Answer:** The upload path bottlenecks are: (1) network bandwidth at the load balancer, (2) chunk reassembly CPU at the upload service, (3) metadata database write throughput. Architecture: clients upload chunks directly to a chunk/blob storage layer (e.g., S3 or GCS) via presigned URLs, bypassing application servers. A separate async service consumes upload-completion events from a queue and updates the metadata store. For metadata sharding, partition by `user_id` (or a hash of `user_id`) across multiple PostgreSQL instances. This keeps all of a user's files co-located for transactional consistency on rename/move operations within their drive. Use read replicas for listing directory contents.

---

## Q3: How does Google Drive handle conflict resolution when a file is modified offline on two devices? Walk through the detection and resolution flow.

**Answer:** When a device comes online, it sends its local revision vector (a list of {file_id, revision_id, timestamp} pairs) to the server. The server compares this against the canonical revision log. If the latest canonical ancestor of both edits diverges (causal conflict), the server detects a conflict. Resolution: the server accepts both versions, marks the file as conflicted, and both copies are stored as separate branches. The user is notified via the UI to manually merge or choose which version to keep. The "winning" version becomes the new canonical head; the losing version is preserved as a conflicted copy. This is a multi-value (non-last-writer-wins) strategy — last-writer-wins would risk data loss.

---

## Q4: Explain the design of the file indexing and search feature in Google Drive. How does it index full-text content of documents without overloading the system?

**Answer:** Files are indexed asynchronously. After a file is uploaded, an event is published to a queue consumed by the indexing service. For text-based files (docs, PDFs, code), the indexing service extracts text content (using OCR for images, PDF parsers, etc.) and builds an inverted index stored in a search engine like Elasticsearch. To avoid overloading: (1) the queue acts as a buffer — indexing can lag behind uploads during peak traffic; (2) file types that are not text-searchable (binaries, videos) are indexed by metadata only (filename, type, size); (3) indexing priority is given to frequently accessed files. The search query path queries Elasticsearch which returns matching file IDs, then joins with the metadata DB for access control filtering.

---

## Q5: Google Drive throttles API requests. Design a rate-limiting strategy that differentiates between interactive user requests and sync client polling.

**Answers:** Use a dual-rate-limiter pattern. Interactive requests (UI operations: rename, delete, move) get a higher-priority token bucket with lower capacity but fast refill — they need low latency. Sync client polling requests (checking for changes) get a separate bucket with higher burst capacity but stricter long-term average because a single sync client can generate far more requests than an interactive user. Distinguish them via API path prefixes or User-Agent headers. Additionally, use a leaky bucket per `user_id` with a global cap per IP to prevent a single misconfigured sync client from starving other users.

---

## Q6: A user has 50,000 files in a single folder. How does Google Drive avoid fetching the entire file list on every sync?

**Answer:** The desktop sync client maintains a local SQLite database that mirrors file metadata. Instead of fetching the full directory tree, the client polls a `/changes` endpoint that returns only the incremental change log (event ID, file ID, operation type, timestamp) since the last known cursor. The cursor is an opaque token that represents the client's sync state. The server maintains a monotonically increasing change ID per user; any mutation (file create, update, delete, share) appends an event. The client fetches batches of changes, applies them to the local SQLite DB, and advances the cursor. This way, listing a folder with 50,000 files requires only O(changes since last sync) work, not O(50,000).

---

## Q7: How does Google Drive ensure data durability against data center failures, and what is the trade-off in cost vs. latency?

**Answer:** Google Drive stores each chunk in multiple geographically distributed data centers (typically 3). Writes are synchronously replicated within the local data center for fast acknowledgment, then asynchronously replicated across regions. For read-after-write consistency within a region, the client reads from the local replica. Cross-region replication is eventual. The trade-off: synchronous cross-region replication would add 50-200 ms of latency per write and triple the write cost, but would guarantee zero data loss. Google chooses async cross-region replication because it keeps write latency low and accepts the minute-scale replication window (during which a full data center loss could lose recent writes). For most file storage use cases, this is acceptable.

---

## Q8: Design the sharing permission model. How does Google Drive evaluate access for a deeply nested file shared with a group?

**Answer:** Permissions are ACL-based. Each file and folder has an ACL (list of {principal, role}). Roles: Owner, Editor, Commenter, Viewer. Inheritance: a file inherits permissions from its parent folder. When checking access for a deeply nested file: (1) the system checks the file's own ACL; if a direct entry matches the user (or any group the user belongs to), it returns that role. (2) If no direct entry, walk up the ancestor chain checking each folder's ACL. (3) Cache the resolved effective permission at the file level with a TTL to avoid O(depth) walks on every access. Group membership is resolved via a group service (e.g., LDAP). ACLs are stored in the metadata DB with an index on `(file_id, principal_id)` for O(1) direct lookup.

---

## Q9: How does Google Drive detect and handle large-scale abuse, such as a script creating millions of files or sharing spam?

**Answer:** Abuse detection operates at multiple layers: (1) Per-user rate limits on file creation, sharing invites, and API calls. (2) Anomaly detection — a user creating 10,000 files per minute triggers a review flag. (3) Heuristic analysis — files shared to too many users in a short window are quarantined pending abuse review. (4) Machine learning classifiers scan file content for malware, phishing, or policy violations. Quarantined files are invisible to recipients, and the uploader is notified with an appeal process. This is implemented as a stream-processing pipeline (e.g., Apache Beam/Dataflow) consuming file creation and share events, running rules and ML models in real-time, and publishing block/allow decisions to a fast key-value store checked on every file access.

---

## Q10: Explain the offline availability feature. How does Google Drive decide which files to cache locally, and how does it handle cache invalidation?

**Answer:** Users can mark specific files or folders as "Available offline". The desktop/mobile client downloads the latest revision of marked files to local storage. For folders, it recursively downloads the subtree. The cache is indexed by file ID and revision ID in the local SQLite DB. Cache invalidation: on the next sync poll, if a file's server revision ID differs from the locally cached one, the client downloads the new chunks and updates the local index. For space management, the client shows the offline cache size and allows the user to remove files. On mobile, the client may limit offline caching to a configurable maximum (e.g., 20 GB) and evict least-recently-used files when space runs low.
