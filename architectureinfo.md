# EU-Social-Search Architecture Documentation

## 1. High-Level Overview

EU-Social-Search is a standalone FastAPI search service for the wider EU social platform. Its main purpose is to serve search results for tools, posts/content, creators, hashtags, and playlists while keeping search-specific concerns separate from the primary social API.

The architecture is service-oriented:

```text
Client
  |
  v
FastAPI application
  |
  v
API routers
  |
  v
SearchManager / IndexSyncService
  |
  +--> Meilisearch for search indexes
  +--> Cassandra for source data and relationship enrichment
  +--> Kafka for analytics and index update events
```

Major modules:

- `EU-Social-Search/app/main.py`: FastAPI app construction, CORS, router registration, startup, and shutdown.
- `EU-Social-Search/app/api/`: HTTP route composition and endpoint handlers.
- `EU-Social-Search/app/core/`: business services for search and index synchronization.
- `EU-Social-Search/app/db/`: Cassandra client and session provider.
- `EU-Social-Search/app/meilisearch/`: Meilisearch client wrapper.
- `EU-Social-Search/app/kafka/`: Kafka producer and consumer integrations.
- `EU-Social/app/db/`: broader production database patterns used by the social backend, including `factory.py`, dependency injection, Cassandra singleton lifecycle, and prepared statement caching.

Design philosophy:

- Keep search as an independent service that can evolve without coupling every search change to the monolithic social API.
- Use singleton provider functions for long-lived infrastructure clients.
- Use Meilisearch as the low-latency read model and Cassandra as the source/enrichment store.
- Use Kafka for asynchronous analytics and index refresh events.
- Prefer simple service objects over deep abstraction in the standalone search service.

Sensitive files such as `.env`, keys, certificates, credentials, and secret manifests were intentionally excluded because they contain sensitive information.

---

## 2. Folder Structure

Important workspace folders:

- `EU-Social-Search/`: standalone search microservice.
- `EU-Social/`: main social backend. Search endpoints are disabled there and moved to the standalone search service, but it contains the richer Cassandra/database foundation.
- `EU-UserApp/`, `EU-Config/`, `EU-Catalogue/`, `EU-AssetManager/`, `EU-Geo/`, `EU-Search/`, `EU-Search-Sync/`, and others: sibling services in the same backend workspace.

Important `EU-Social-Search` folders:

- `app/`: application source code.
- `app/api/`: API router registration and endpoint modules.
- `app/api/endpoints/`: concrete HTTP handlers for search, SuperApp search, and index management.
- `app/core/`: business logic. `search_manager.py` owns search operations; `index_sync.py` synchronizes Cassandra records into Meilisearch.
- `app/db/`: Cassandra connection wrapper.
- `app/meilisearch/`: Meilisearch client wrapper.
- `app/kafka/`: Kafka producer for analytics/events and async consumer for index update messages.
- `app/scripts/`: operational scripts for indexing tools.
- `config/`: settings object. Sensitive runtime values are expected to come from environment files, which were excluded.
- root docs such as `README.md`, `QUICKSTART.md`, and `SUPERAPP_SEARCH_API.md`: service documentation.

Important `EU-Social` folders used for architectural context:

- `app/api/`: FastAPI application factory, routers, lifespan, exception handlers.
- `app/api/v1/`: v1 feature modules such as feed, content, followers, likes, comments, saved content, and search placeholder.
- `app/api/v2/`: v2 routes.
- `app/db/`: PostgreSQL factory, Cassandra singleton client, prepared statements, Cassandra utility helpers, SQLAlchemy base and metadata.
- `app/cache/`: Redis factory, dependency, decorators, cache service.
- `app/core/`: logging, middleware, Celery app, constants, exceptions.
- `app/tasks/`: background jobs.
- `app/utils/`: external service clients, Kafka, feed helpers, repositories, auth utilities.
- `tests/`: automated tests and verification scripts.

---

## 3. Complete Request Lifecycle

Typical search request:

```text
Client
  |
  | GET /v1/superapp/search/?query=term
  v
EU-Social-Search/app/main.py
  |
  | app.include_router(api_router, prefix="/v1")
  v
app/api/router.py
  |
  | prefix="/superapp/search"
  v
app/api/endpoints/superapp_search.py
  |
  | Depends(get_search_manager)
  v
SearchManager singleton
  |
  +--> Meilisearch index search
  +--> Kafka analytics event
  +--> Cassandra enrichment for follow status when needed
  |
  v
Endpoint response formatter
  |
  v
JSON response to client
```

Typical index sync request:

```text
Client/Admin
  |
  | POST /v1/index/sync?index_type=content&document_id=...
  v
index endpoint
  |
  | Depends(get_index_sync_service)
  v
IndexSyncService singleton
  |
  +--> Cassandra read
  +--> Transform row to search document
  +--> SearchManager.update_*_index()
  +--> Meilisearch add/delete document
  +--> Kafka index event
  |
  v
JSON sync status
```

The standalone service does not implement a strict Router -> Service -> Repository stack. Instead, route handlers call service objects directly. Repository-like behavior exists in the wider social backend, for example `EU-Social/app/utils/draft_repository.py` and service methods that encapsulate Cassandra access.

---

## 4. FastAPI Startup Lifecycle

In `EU-Social-Search/app/main.py`:

- Logging is configured at import time from settings.
- `FastAPI(...)` is constructed with title, description, version, and debug flag.
- CORS middleware is registered.
- `api_router` is included under `/v1`.
- Startup event logs service metadata and starts the Kafka index update consumer if available.
- Shutdown event stops the Kafka consumer.

Startup diagram:

```text
Process starts
  |
  v
Import app.main
  |
  +--> configure logging
  +--> create FastAPI app
  +--> add CORS middleware
  +--> include /v1 router
  |
  v
startup_event()
  |
  +--> get_consumer()
  +--> consumer.start()
  +--> background consume task begins
```

In `EU-Social/app/api/application.py`, the broader social backend uses the newer FastAPI lifespan pattern:

- `get_app()` configures logging.
- Creates the app with `lifespan=lifespan_setup`.
- Registers Sentry when enabled.
- Registers CORS, exception handlers, logging middleware, v1/v2 routers, health route, redirect router, and static docs.

---

## 5. Dependency Injection Flow

Standalone search service dependencies:

- `get_search_manager()` returns a process-wide `SearchManager` singleton.
- `get_index_sync_service()` returns a process-wide `IndexSyncService` singleton.
- `get_meilisearch_client()` returns a process-wide Meilisearch wrapper.
- `get_kafka_producer()` returns a process-wide Kafka producer wrapper.
- `get_cassandra_session()` returns a Cassandra session from a singleton Cassandra client.

Flow:

```text
FastAPI endpoint
  |
  | Depends(get_search_manager)
  v
get_search_manager()
  |
  | creates SearchManager once
  v
SearchManager.__init__()
  |
  +--> get_meilisearch_client()
  +--> get_kafka_producer()
  +--> get_cassandra_session()
```

In `EU-Social`, dependency injection is more explicit:

- `app.state.db_factory` is created during lifespan startup.
- `get_db_session(request)` reads `request.app.state.db_factory`, yields an `AsyncSession`, commits, and closes it.
- `get_cassandra_session()` returns the Cassandra singleton session.
- Cache dependencies provide Redis clients from factories stored on `app.state`.

---

## 6. Database Architecture

Supported data stores found in the architecture:

- Cassandra: primary social data, content metadata, followers/following, likes, saved content, comments, reports, feeds, and search enrichment.
- Meilisearch: search read model for content, creators, hashtags, playlists, and tools.
- Kafka: not a database, but part of persistence/event flow for analytics and index updates.
- PostgreSQL via SQLAlchemy async engine in `EU-Social/app/db/factory.py`.
- Redis cache in the wider `EU-Social` backend.

Current search-service implementation:

- Search reads are served by Meilisearch.
- Some creator result enrichment uses Cassandra to compute `is_following`.
- Index sync reads Cassandra rows and writes transformed documents to Meilisearch.
- Kafka records search analytics and index events.

Abstraction:

- `MeiliSearchClient` wraps the Meilisearch SDK.
- `CassandraClient` wraps cluster/session creation and reuse.
- `KafkaProducer` wraps serialization and producer calls.
- `DatabaseFactory` in `EU-Social` abstracts SQLAlchemy async engine/session creation.

Connection flow:

```text
SearchManager
  |
  +--> MeiliSearchClient.get_client()
  +--> KafkaProducer._initialize()
  +--> CassandraClient.connect()
```

---

## 7. Cassandra Architecture

Standalone `EU-Social-Search/app/db/cassandra_client.py`:

- `CassandraClient` stores `_cluster` and `_session` on the instance.
- `connect()` returns the existing session if present.
- On first connection, hosts are parsed from settings, `Cluster(...)` is created, `cluster.connect()` creates a session, and `set_keyspace(...)` selects the keyspace.
- `execute()` calls `connect()` and then `session.execute(...)`.
- `disconnect()` shuts down the cluster and clears both `_cluster` and `_session`.
- `get_cassandra_client()` creates one global client object.
- `get_cassandra_session()` returns the singleton client session.

`EU-Social/app/db/cassandra_client.py` uses class-level state:

- `_session` and `_cluster` are class variables.
- `get_session()` creates the cluster/session once, optionally configures authentication, creates the keyspace if missing, and sets the keyspace.
- `close()` shuts down the cluster and clears singleton state.

Cassandra lifecycle diagram:

```text
First dependency call
  |
  v
get_cassandra_session()
  |
  v
CassandraClient.get_session/connect()
  |
  | no existing session
  v
Create Cluster
  |
  v
cluster.connect()
  |
  v
session.set_keyspace(...)
  |
  v
Store session for reuse
```

Session reuse:

```text
Request A --> get_cassandra_session() --> existing Session
Request B --> get_cassandra_session() --> same Session
Request C --> get_cassandra_session() --> same Session
```

Thread safety:

- The DataStax Cassandra driver sessions are designed to be long-lived and reused.
- The current singleton creation path is simple and generally fine after startup.
- `EU-Social` prepared statement cache uses a lock in `FeedPreparedStatements` for concurrent lazy preparation.
- `EU-Social-Search` does not lock around first Cassandra client/session creation, so highly concurrent first use could theoretically attempt duplicate creation. In practice, this is reduced when initialization happens during service construction.

Shutdown:

- `EU-Social` explicitly calls `CassandraClient.close()` in lifespan shutdown.
- `EU-Social-Search` defines `disconnect()` but the current app shutdown handler only stops the Kafka consumer. Cassandra shutdown would be a recommended improvement.

---

## 8. Factory Pattern

`EU-Social-Search` does not contain a `factory.py`; it uses singleton provider functions instead.

Factory pattern in `EU-Social/app/db/factory.py`:

- `DatabaseFactory(db_url, db_echo)` creates an async SQLAlchemy engine.
- It creates an `async_sessionmaker`.
- `get_session()` returns a new `AsyncSession`.
- `close()` disposes the engine.

Returned objects:

- `DatabaseFactory.engine`: shared async engine.
- `DatabaseFactory.session_factory`: session factory.
- `get_session()`: per-request session.

Lifecycle:

```text
lifespan startup
  |
  v
DatabaseFactory(...)
  |
  v
app.state.db_factory
  |
  v
get_db_session() per request
  |
  v
commit + close
  |
  v
lifespan shutdown disposes engine
```

The search-service equivalents are:

- `get_search_manager()`
- `get_index_sync_service()`
- `get_meilisearch_client()`
- `get_kafka_producer()`
- `get_cassandra_client()`

These are singleton providers rather than factories in the strict sense.

---

## 9. Repository Pattern

The standalone search service mostly uses service classes directly:

- `SearchManager` combines search orchestration, analytics, enrichment, and index writes.
- `IndexSyncService` reads Cassandra and writes Meilisearch documents.

Repository-like separation appears in the wider `EU-Social` backend:

- `EU-Social/app/utils/draft_repository.py` encapsulates draft persistence in PostgreSQL.
- `EU-Social/app/api/v1/saved_content/service.py` encapsulates saved-content Cassandra operations.
- `EU-Social/app/api/v1/content/service.py`, feed helpers, and other feature services isolate database access from routers.

Current separation of concerns:

```text
Router/view
  |
  v
Service or repository-like helper
  |
  v
Database/query client
```

The search service would benefit from extracting Cassandra reads into repositories if database usage grows.

---

## 10. Prepared Statements

Prepared statements are most explicit in `EU-Social/app/db/cassandra_prepared_stmt.py`.

Preparation:

- `prepare_all_query_statements(session)` runs during `EU-Social` lifespan startup.
- It prepares commonly used queries from `CassandraQueries`.
- Prepared statements are stored as class variables on `PreparedStatements`.

Caching:

- `PreparedStatements` caches common statements such as saved content, blocked users, and user profiles.
- `FeedPreparedStatements` caches feed statements by `(id(session), query)` and protects lazy preparation with a `threading.Lock`.

Execution flow:

```text
Startup
  |
  v
prepare_all_query_statements(session)
  |
  v
PreparedStatements.GET_SAVED_CONTENTS = session.prepare(...)

Request
  |
  v
PreparedStatements.get_saved_contents(session)
  |
  v
session.execute(prepared_stmt, params)
```

In `EU-Social-Search`, queries are executed directly. It does not currently centralize prepared statement caching.

---

## 11. Database Models

`EU-Social` uses SQLAlchemy models and metadata:

- `app/db/base.py` defines `Base` from `DeclarativeBase`.
- `app/db/meta.py` holds SQLAlchemy metadata.
- `app/db/models/` contains model package initialization.
- Alembic migrations live under `app/db/migrations/`.

Cassandra models:

- Cassandra data is represented primarily through table-oriented CQL in `CassandraQueries`.
- Rows returned by the Cassandra driver are transformed into dictionaries or Pydantic response schemas in services.
- The codebase uses denormalized Cassandra tables such as `content_by_id`, `content_by_user`, `followers_by_user`, `following_by_user`, `likes_by_content`, `saved_content_by_user`, `comments_by_content_v2`, and feed/list tables.

Serialization:

- API schemas live in modules such as `EU-Social/app/api/v1/schemas.py` and feed/search schema files.
- `EU-Social-Search` directly returns dictionaries shaped for client compatibility with prior EU-Social/SuperApp search responses.

---

## 12. Startup Flow

`EU-Social-Search` startup until first request:

```text
1. Python imports app.main.
2. Settings are loaded.
3. Logging is configured.
4. FastAPI app is created.
5. CORS middleware is attached.
6. API router is included at /v1.
7. FastAPI startup_event runs.
8. Kafka index consumer singleton is created.
9. Consumer connects and starts background consumption.
10. First request resolves endpoint dependencies.
11. SearchManager or IndexSyncService singleton is created lazily.
12. Meilisearch, Kafka producer, and Cassandra session are initialized lazily.
13. Request is processed.
```

`EU-Social` startup:

```text
1. get_app() constructs FastAPI app with lifespan_setup.
2. Logging, Sentry, middleware, exception handlers, routers, and static files are configured.
3. lifespan_setup creates DatabaseFactory.
4. Redis factories are created and optionally pre-initialized.
5. Factories are stored in app.state.
6. Cassandra session is created.
7. Cassandra tables/utilities are ensured.
8. Prepared statements are initialized.
9. Kafka user profile consumer starts.
10. App begins serving traffic.
```

---

## 13. Shutdown Flow

`EU-Social-Search` shutdown:

```text
FastAPI shutdown_event
  |
  v
get_consumer()
  |
  v
consumer.stop()
  |
  +--> cancel consume task
  +--> stop AIOKafkaConsumer
```

`EU-Social` shutdown:

```text
lifespan finally
  |
  +--> close PostgreSQL DatabaseFactory
  +--> close Redis factory
  +--> close OAuth Redis factory
  +--> stop Kafka consumer
  +--> CassandraClient.close()
```

Recommended gap for `EU-Social-Search`: call Cassandra `disconnect()` and Kafka producer `close()` during shutdown.

---

## 14. Database Client Lifecycle

Where clients are created:

- Meilisearch: `EU-Social-Search/app/meilisearch/client.py`, inside `MeiliSearchClient.get_client()`.
- Kafka producer: `EU-Social-Search/app/kafka/producer.py`, inside `KafkaProducer.__init__()`.
- Kafka consumer: `EU-Social-Search/app/kafka/consumer.py`, inside `MeilisearchIndexConsumer.start()`.
- Cassandra: `EU-Social-Search/app/db/cassandra_client.py`, inside `CassandraClient.connect()`.
- PostgreSQL: `EU-Social/app/db/factory.py`, inside `DatabaseFactory.__init__()`.
- Redis: `EU-Social/app/api/lifespan.py`, inside `_init_redis_factories()`.

Where clients are stored:

- Search service clients are stored in module-level singleton variables.
- Social backend SQL/Redis factories are stored on `app.state`.
- Social backend Cassandra client is stored as class-level `_cluster` and `_session`.

Ownership:

- The process owns singleton infrastructure clients.
- FastAPI request handlers borrow these clients through dependencies.
- Per-request SQLAlchemy sessions are owned by the request dependency and closed after use.

Destruction:

- Search service destroys Kafka consumer on app shutdown.
- Social backend destroys PostgreSQL, Redis, Kafka consumer, and Cassandra during lifespan shutdown.

---

## 15. Current Persistent Connection Strategy

Cassandra avoids reconnecting per request by holding a long-lived session.

`EU-Social-Search`:

```text
_cassandra_client = None
  |
  v
get_cassandra_client()
  |
  | creates CassandraClient once
  v
CassandraClient._session
  |
  | created once by connect()
  v
All future calls reuse same Session
```

`EU-Social`:

```text
CassandraClient._session class variable
  |
  | None at process start
  v
get_session()
  |
  | creates Cluster and Session once
  v
class-level session reused across requests
```

Request reuse diagram:

```text
Request 1 --> get_cassandra_session() --> create session --> execute query
Request 2 --> get_cassandra_session() --> reuse session  --> execute query
Request 3 --> get_cassandra_session() --> reuse session  --> execute query
```

This strategy is appropriate for Cassandra because sessions are intended to be shared, long-lived, and internally manage connection pools to cluster nodes.

---

## 16. Important Design Patterns

- Singleton: module-level singletons in `EU-Social-Search` for search manager, sync service, Meilisearch client, Kafka producer, Kafka consumer, and Cassandra client.
- Factory: `EU-Social/app/db/factory.py` creates SQLAlchemy engines and sessions; Redis factories create cache clients.
- Dependency Injection: FastAPI `Depends(...)` injects `SearchManager`, `IndexSyncService`, Cassandra sessions, Redis clients, and database sessions.
- Repository: repository-like persistence modules such as `draft_repository.py` and feature services that own DB access.
- Service Layer: `SearchManager`, `IndexSyncService`, `SavedContentService`, content/feed services.
- Adapter/Wrapper: `MeiliSearchClient`, `KafkaProducer`, `CassandraClient`, `RedisFactory`.
- Strategy: search endpoints choose different search strategies based on query flags such as `creators`, `hashtags`, `posts`, `playlists`, and `top_results`.
- Read Model / CQRS-like pattern: Cassandra is the source/enrichment store, while Meilisearch is a denormalized read model for search.
- Prepared Statement Cache: `PreparedStatements` and `FeedPreparedStatements`.
- Lifespan Resource Management: `EU-Social` uses FastAPI lifespan for startup/shutdown ownership.

---

## 17. Extension Points

To add a new database to the standalone search service:

Files likely to change:

- `EU-Social-Search/config/settings.py`: add non-sensitive configuration fields.
- New module under `EU-Social-Search/app/db/`: implement the new client wrapper.
- `EU-Social-Search/app/core/search_manager.py`: inject/use the new client only where search enrichment requires it.
- `EU-Social-Search/app/core/index_sync.py`: read/write through the new database if it participates in indexing.
- `EU-Social-Search/app/main.py`: initialize and close long-lived clients if eager startup/shutdown is preferred.
- Tests and docs for the new behavior.

To add a new database to the broader `EU-Social` backend:

- Add a factory/client module under `EU-Social/app/db/`.
- Register it in `EU-Social/app/api/lifespan.py`.
- Store it on `app.state` if it has process lifecycle ownership.
- Add a dependency function in `EU-Social/app/db/dependencies.py` or a new dependency module.
- Add repositories/services that use the dependency.
- Add shutdown cleanup in lifespan `finally`.

Preferred extension shape:

```text
settings
  |
  v
client/factory module
  |
  v
lifespan startup stores app.state or singleton
  |
  v
dependency provider
  |
  v
service/repository
  |
  v
router
```

---

## 18. Google Cloud Bigtable Integration Analysis

Components that should remain unchanged:

- FastAPI routers and route contracts.
- Response schemas and client-facing payload shapes.
- Meilisearch query path for search results.
- Kafka analytics flow.
- Existing Cassandra-backed behavior until Bigtable is explicitly selected.

Components that should change:

- Add a Bigtable client wrapper/factory.
- Add settings for project, instance, table names, and emulator usage if needed.
- Add a repository layer for Bigtable reads/writes instead of placing Bigtable logic directly inside endpoint handlers.
- Extend `IndexSyncService` to read from Bigtable when the indexed source moves from Cassandra to Bigtable.

Where a Bigtable client should be initialized:

- For `EU-Social-Search`, initialize it as a singleton provider under `app/db/bigtable_client.py`, and close it during FastAPI shutdown.
- For `EU-Social`, initialize it in `lifespan_setup`, store it on `app.state`, and expose it through a dependency.

Singleton client recommendation:

- Use a singleton or app-lifespan-owned Bigtable client per process.
- Do not create a Bigtable client per request.
- Bigtable clients maintain network channels and are designed for reuse.

Connection reuse:

```text
Application startup or first use
  |
  v
Create Bigtable client
  |
  v
Open instance/table handles
  |
  v
Reuse for all repository calls
  |
  v
Close on shutdown
```

How the existing architecture can support Bigtable:

- The existing singleton/factory/provider style maps cleanly to Bigtable.
- `IndexSyncService` can be made source-agnostic by depending on a repository interface.
- Search routes can remain unchanged because they already depend on `SearchManager`, not directly on Cassandra.
- Cassandra-specific enrichment, such as follow status, can be moved behind a social graph repository with Cassandra and Bigtable implementations.

Recommended production architecture:

```text
Router
  |
  v
SearchManager
  |
  +--> SearchIndexRepository (Meilisearch)
  +--> SocialGraphRepository (Cassandra or Bigtable)
  +--> AnalyticsPublisher (Kafka)

IndexSyncService
  |
  +--> ContentSourceRepository (Cassandra or Bigtable)
  +--> SearchIndexRepository
```

This keeps Bigtable from leaking into API handlers and allows controlled migration from Cassandra.

---

## 19. Sequence Diagrams

Startup:

```text
FastAPI        app.main        KafkaConsumer
  |               |                 |
  | import app    |                 |
  |-------------->|                 |
  |               | create app      |
  |               | register router |
  | startup       |                 |
  |-------------->| get_consumer()  |
  |               |---------------->|
  |               | start()         |
  |               |---------------->|
  |               | background task |
```

Read request:

```text
Client      Router      SearchManager      Meilisearch      Cassandra      Kafka
  |           |              |                 |              |             |
  | GET       |              |                 |              |             |
  |---------->| Depends      |                 |              |             |
  |           |------------->|                 |              |             |
  |           | search_*     |                 |              |             |
  |           |------------->| send analytics  |              |------------>|
  |           |              | search index    |------------->|             |
  |           |              | enrich follows  |              |------------>|
  |           | response     |                 |              |             |
  |<----------|              |                 |              |             |
```

Database query:

```text
Service          CassandraClient          Cluster/Session
  |                    |                         |
  | get session        |                         |
  |------------------->|                         |
  |                    | existing session?       |
  |                    | yes: return             |
  |                    | no: create cluster      |
  |                    |------------------------>|
  | execute CQL        |                         |
  |------------------->| session.execute(...)    |
  |                    |------------------------>|
  | rows               |                         |
  |<-------------------|                         |
```

Shutdown:

```text
FastAPI        Shutdown Handler        KafkaConsumer        CassandraClient
  |                   |                      |                    |
  | shutdown          |                      |                    |
  |------------------>| get_consumer()       |                    |
  |                   | stop()               |------------------->|
  |                   | cancel task          |                    |
  |                   | stop consumer        |                    |
  |                   | recommended close    |------------------->|
```

---

## 20. Call Graph

Typical `GET /v1/superapp/search/?query=x` call graph:

```text
app.main.app
  -> app.api.router.api_router
    -> app.api.endpoints.superapp_search.superapp_search()
      -> get_search_manager()
        -> SearchManager.__init__() on first use
          -> get_meilisearch_client()
          -> get_kafka_producer()
          -> get_cassandra_session()
      -> SearchManager.search_tools()
        -> _send_analytics()
        -> KafkaProducer.send()
        -> MeiliSearchClient.get_index()
        -> Index.search()
      -> SearchManager.search_content()
        -> _send_analytics()
        -> Index.search()
      -> SearchManager.search_creators()
        -> _send_analytics()
        -> Index.search()
        -> Cassandra session.execute() for follow status when user id exists
      -> SearchManager.search_hashtags()
        -> _send_analytics()
        -> Index.search()
      -> response dictionary
```

Typical `POST /v1/index/sync` call graph:

```text
index.sync_index()
  -> get_index_sync_service()
    -> IndexSyncService.__init__()
      -> get_search_manager()
      -> get_cassandra_session()
  -> sync_content_to_index() / sync_creator_to_index() / sync_hashtag_to_index()
    -> Cassandra session.execute()
    -> transform row to document
    -> SearchManager.update_*_index()
      -> MeiliSearchClient.get_index()
      -> Index.add_documents() or delete_document()
      -> SearchManager._send_index_event()
        -> KafkaProducer.send()
```

---

## 21. File Dependency Graph

Standalone search service:

```text
app/main.py
  -> app/api/router.py
  -> config/settings.py
  -> app/kafka/consumer.py

app/api/router.py
  -> app/api/endpoints/search.py
  -> app/api/endpoints/superapp_search.py
  -> app/api/endpoints/index.py

search.py, superapp_search.py
  -> app/core/search_manager.py

index.py
  -> app/core/index_sync.py

app/core/search_manager.py
  -> app/db/cassandra_client.py
  -> app/kafka/producer.py
  -> app/meilisearch/client.py
  -> config/settings.py

app/core/index_sync.py
  -> app/core/search_manager.py
  -> app/db/cassandra_client.py

app/kafka/consumer.py
  -> app/meilisearch/client.py
  -> config/settings.py
```

Broader social backend database foundation:

```text
app/api/application.py
  -> app/api/lifespan.py
  -> app/api/v1/router.py
  -> app/api/v2/router.py
  -> middleware / exception handlers / logging

app/api/lifespan.py
  -> app/db/factory.py
  -> app/db/cassandra_client.py
  -> app/db/cassandra_prepared_stmt.py
  -> app/cache/factory.py
  -> kafka consumer

app/db/dependencies.py
  -> request.app.state.db_factory

feature routers/services
  -> app/api/queries.py
  -> app/db/cassandra_client.py
  -> app/db/cassandra_prepared_stmt.py
```

---

## 22. Important Files

API:

- `EU-Social-Search/app/main.py`: app creation, CORS, router registration, startup/shutdown.
- `EU-Social-Search/app/api/router.py`: combines search, SuperApp search, and index routers.
- `EU-Social-Search/app/api/endpoints/search.py`: unified search endpoint.
- `EU-Social-Search/app/api/endpoints/superapp_search.py`: SuperApp-compatible search endpoint with category-specific behavior.
- `EU-Social-Search/app/api/endpoints/index.py`: index sync, batch sync, and full reindex endpoints.
- `EU-Social/app/api/application.py`: production social backend app factory.
- `EU-Social/app/api/lifespan.py`: production startup/shutdown resource ownership.

Database:

- `EU-Social-Search/app/db/cassandra_client.py`: Cassandra singleton client for search service.
- `EU-Social/app/db/cassandra_client.py`: class-level Cassandra singleton, keyspace creation, shutdown.
- `EU-Social/app/db/factory.py`: SQLAlchemy async database factory.
- `EU-Social/app/db/dependencies.py`: per-request SQL session dependency.
- `EU-Social/app/db/cassandra_prepared_stmt.py`: prepared statement initialization and cache.
- `EU-Social/app/api/queries.py`: central CQL and SQL query definitions.

Repository / service:

- `EU-Social-Search/app/core/search_manager.py`: core search orchestration.
- `EU-Social-Search/app/core/index_sync.py`: Cassandra-to-Meilisearch sync service.
- `EU-Social/app/utils/draft_repository.py`: PostgreSQL draft repository.
- `EU-Social/app/api/v1/saved_content/service.py`: saved-content service and Cassandra batch operations.
- `EU-Social/app/api/v1/content/service.py`: content business logic and persistence operations.

Factory / Dependency Injection:

- `EU-Social/app/db/factory.py`: database factory.
- `EU-Social/app/db/dependencies.py`: SQL session dependency.
- `EU-Social-Search/app/core/search_manager.py`: `get_search_manager()` singleton provider.
- `EU-Social-Search/app/core/index_sync.py`: `get_index_sync_service()` singleton provider.
- `EU-Social-Search/app/meilisearch/client.py`: `get_meilisearch_client()` singleton provider.
- `EU-Social-Search/app/kafka/producer.py`: `get_kafka_producer()` singleton provider.

Startup / Configuration:

- `EU-Social-Search/config/settings.py`: settings declarations. Sensitive runtime values are intentionally not documented.
- `EU-Social/app/settings.py`: social backend settings declarations. Sensitive runtime values are intentionally not documented.
- Environment files: intentionally excluded because they contain sensitive information.

---

## 23. Potential Improvement Opportunities

- Add explicit Cassandra and Kafka producer cleanup to `EU-Social-Search` shutdown.
- Move `EU-Social-Search` from deprecated `@app.on_event` startup/shutdown to FastAPI lifespan for consistent resource ownership.
- Add a repository layer for Cassandra reads in the search service.
- Add prepared statement caching to the standalone search service for repeated Cassandra enrichment queries.
- Add locking or eager startup initialization for singleton clients to avoid duplicate first-use initialization under concurrency.
- Separate analytics publishing from `SearchManager` into an `AnalyticsPublisher`.
- Separate Meilisearch operations into a `SearchIndexRepository`.
- Make `IndexSyncService` source-agnostic so Cassandra, Bigtable, or another source can be swapped with less change.
- Avoid returning raw exception strings to clients in production responses.
- Add health checks that validate Meilisearch, Cassandra, and Kafka readiness separately.
- Add integration tests for index sync and SuperApp response compatibility.
- Standardize query parameter naming; some endpoints use `filter_type` while examples refer to `filter`.
- Avoid full reindex endpoints being exposed without strong authorization controls.

---

## 24. Summary

EU-Social-Search is the dedicated search microservice for the EU social platform. It receives FastAPI requests, routes them to search endpoints, uses a singleton `SearchManager` to query Meilisearch, enriches selected results from Cassandra, emits analytics through Kafka, and returns client-compatible response shapes.

The broader `EU-Social` service provides the mature database architecture around this ecosystem: FastAPI lifespan resource ownership, PostgreSQL factory/session management, Redis factories, Cassandra singleton sessions, prepared statement caching, and feature-level service/repository patterns.

The current persistent Cassandra strategy is based on long-lived singleton sessions, which avoids reconnecting for each request. Meilisearch acts as the search read model, Cassandra remains the source/enrichment store, and Kafka carries analytics and index update events. The architecture can support Bigtable by adding a lifecycle-managed client and moving Cassandra-specific reads behind repository interfaces while leaving routers and response contracts largely unchanged.
