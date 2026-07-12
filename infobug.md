# Templates API `user_info` Null Fields Investigation

## Scope And Safety

This analysis is based only on the local codebase. No credentials, secrets, tokens, passwords, certificates, connection strings, environment variable values, or sensitive configuration values were inspected or included.

Requested public API:

```http
GET /creator-economy/v1/templates/{template_id}
```

Important repository finding: this exact route string is not declared in the local `EU-Social` FastAPI codebase. The app registers routes under `/social/v1`, and the relevant implementation for non-UGC/static/template-like content appears to be the feed content-listing path:

```http
GET /social/v1/feed/content_listing?name=...&content_id=...
```

The code path is used for static list content such as top/highlight/explore content and returns assets with `user_info`. It is the local implementation that matches the reported Today's Highlights / Explore behavior.

## 1. Route Handling

| Concern | Local implementation |
|---|---|
| App route prefix | `app.include_router(router=api_router, prefix="/social/v1")` in `EU-Social/app/api/application.py` |
| Feed router registration | `api_router.include_router(feed.router, prefix="/feed", tags=["Feeds"])` in `EU-Social/app/api/v1/router.py` |
| Relevant route | `@router.get("/content_listing")` in `EU-Social/app/api/v1/feed/views.py` |
| Handler | `get_content_listing(...)` |
| Public route requested | Not found locally; likely gateway/proxy alias or missing route in this repository |

No local FastAPI handler for `/templates/{template_id}` was found.

## 2. Complete Execution Flow

For the local relevant static/template content path:

```text
HTTP request
  -> FastAPI app in app/api/application.py
  -> app/api/v1/router.py includes feed router at /social/v1/feed
  -> app/api/v1/feed/views.py:get_content_listing
  -> CassandraQueries.GET_TOP_CONTENT_BY_LIST_NAME
  -> social_keyspace.top_content_list
  -> ContentService.get_multiple_content_details
  -> CassandraQueries.GET_MULTIPLE_CONTENT_DETAILS
  -> social_keyspace.content_by_id
  -> ContentService.format_content_data
  -> ContentService.enrich_content_items
  -> ContentService._fetch_user_map
  -> CassandraQueries.GET_USERS_PROFILES
  -> social_keyspace.user_profile
  -> app/api/v1/feed/schemas.py:TopContent
  -> TopContent.model_dump()
  -> standard_response(...)
  -> JSONResponse
```

For the Social UGC content-by-id path:

```text
HTTP request
  -> FastAPI app in app/api/application.py
  -> app/api/v1/router.py includes content router
  -> app/api/v1/content/views.py:get_content
  -> ContentService.increment_view_count
  -> ContentService.get_content_details
  -> CassandraQueries.GET_CONTENT_DETAILS
  -> social_keyspace.content_by_id
  -> ContentService.format_content_data
  -> ContentService.enrich_content_items
  -> ContentService._fetch_user_map
  -> CassandraQueries.GET_USERS_PROFILES
  -> social_keyspace.user_profile
  -> app/api/v1/schemas.py:ContentByIdData
  -> ContentByIdData.model_dump()
  -> standard_response(...)
  -> JSONResponse
```

## 3. Files Involved

| Relative path | Role |
|---|---|
| `EU-Social/app/api/application.py` | Creates FastAPI app and mounts `api_router` under `/social/v1`. |
| `EU-Social/app/api/v1/router.py` | Includes `content.router` and `feed.router`. |
| `EU-Social/app/api/v1/feed/views.py` | Contains static content listing route and feed enrichment helpers. |
| `EU-Social/app/api/v1/feed/schemas.py` | Defines `UserInfo`, `TrayAsset`, `TopContent`, and related response models. |
| `EU-Social/app/api/v1/content/views.py` | Contains UGC content-by-id and user-content routes. |
| `EU-Social/app/api/v1/content/service.py` | Fetches content rows, enriches counters/interactions/user details, builds `user_info`. |
| `EU-Social/app/api/v1/schemas.py` | Defines UGC response models including `ContentCreatorInfo` and `ContentByIdData`. |
| `EU-Social/app/api/queries.py` | Contains Cassandra CQL query constants. |
| `EU-Social/app/utils/standard_response.py` | Wraps payload in the standard JSON response. |
| `EU-Social/app/db/cassandra_utils.py` | Defines/syncs `user_profile`, including `country_code` and `created_at`. |
| `EU-Social/app/core/constants.py` | Defines a `COUNTRY_CODE` constant. |

## 4. How The Response JSON Is Built

`get_content_listing` builds the final JSON in these steps:

1. Validates the `name` query parameter.
2. Resolves the optional viewer ID from headers.
3. Builds a cache key and returns cached response if present.
4. Reads static list rows from `social_keyspace.top_content_list`.
5. Optionally filters rows by `content_id`.
6. Paginates the static list rows.
7. Reads content details from `social_keyspace.content_by_id` using `ContentService.get_multiple_content_details`.
8. Formats each Cassandra content row through `ContentService.format_content_data`.
9. Enriches formatted content with counters, social flags, and `user_info` through `ContentService.enrich_content_items`.
10. Maps each enriched dict into `TopContent`.
11. Serializes each model using `c.model_dump()`.
12. Wraps the data under:

```python
{
    "trays": [
        {
            "assets": [c.model_dump() for c in top_content_list],
        },
    ],
    "next": next_offset,
    "previous": previous_offset,
}
```

13. `standard_response(...)` wraps this in:

```python
{
    "success": True,
    "message": "...",
    "data": jsonable_encoder(data),
    "meta": {...},
    "error": {},
}
```

## 5. Where `user_info` Is Created

The main reusable implementation is:

```python
# EU-Social/app/api/v1/content/service.py
user_info = user_map.get(
    uid,
    {
        "id": uid,
        "username": "unknown",
        "profile_picture": None,
        "country": None,
        "created_at": None,
    },
)
user_info["is_following"] = uid in following_ids
item["user_info"] = user_info
```

`user_map` is created in `ContentService._fetch_user_map`:

```python
user_map[user.user_id] = {
    "id": user.user_id,
    "username": user_name,
    "profile_picture": getattr(user, "image_url", None),
    "country": getattr(user, "country_code", None),
    "created_at": getattr(user, "created_at", None),
}
```

There is also a separate feed-only creator summary path:

```python
# EU-Social/app/api/v1/feed/views.py
user_info_obj = UserInfo(
    id=uid,
    username=u_info["username"],
    profile_picture=u_info["profile_picture"],
)
```

That feed-only path currently omits both `created_at` and `country_code`.

## 6. Social UGC Implementation Returning The Underlying Data

The UGC enrichment path already fetches the source values:

| Field requested | Existing source in UGC helper | Current exposed key |
|---|---|---|
| `profile_created_at` | `getattr(user, "created_at", None)` | `created_at` |
| `country_code` | `getattr(user, "country_code", None)` | `country` |

Relevant files:

| File | Function/class | Detail |
|---|---|---|
| `EU-Social/app/api/v1/content/service.py` | `ContentService._fetch_user_map` | Fetches `created_at` and `country_code` from `user_profile`. |
| `EU-Social/app/api/v1/content/service.py` | `ContentService.enrich_content_items` | Attaches `user_info` to each content item. |
| `EU-Social/app/api/v1/schemas.py` | `ContentCreatorInfo` | Currently exposes only `id`, `username`, `profile_picture`, `is_following`. |
| `EU-Social/app/api/v1/schemas.py` | `ContentByIdData` | Uses `ContentCreatorInfo` for `user_info`. |

Note: The local UGC service already retrieves the values, but under `created_at` and `country`. The requested output names, `profile_created_at` and `country_code`, are not present in the schema.

## 7. Templates API vs Social API Comparison

| Area | Static/template content listing | Social UGC content |
|---|---|---|
| Route in local code | `/social/v1/feed/content_listing` | `/social/v1/content/{content_id}` and `/social/v1/content/user/` |
| Content lookup | `top_content_list` -> `content_by_id` | `content_by_id` or `content_by_user` -> `content_by_id` |
| Enrichment helper | `ContentService.enrich_content_items` | `ContentService.enrich_content_items` |
| User profile query | `GET_USERS_PROFILES` | `GET_USERS_PROFILES` |
| User profile source | `social_keyspace.user_profile` | `social_keyspace.user_profile` |
| Raw profile created field fetched | Yes, as `created_at` in `_fetch_user_map` | Yes, as `created_at` in `_fetch_user_map` |
| Raw country field fetched | Yes, as `country` in `_fetch_user_map` | Yes, as `country` in `_fetch_user_map` |
| Requested output names in response model | No | No local schema field with those exact names |

The two paths share the same reusable content enrichment helper for static-list details. The missing values are caused by name mismatch and schema omission, not by the Cassandra profile query.

## 8. Why `profile_created_at` And `country_code` Are Missing

The fields are missing because:

1. `ContentService._fetch_user_map` fetches the underlying data but stores it as:
   - `created_at`
   - `country`
2. The requested response keys are:
   - `profile_created_at`
   - `country_code`
3. The Pydantic response models do not define either requested key:
   - `app/api/v1/feed/schemas.py:UserInfo`
   - `app/api/v1/schemas.py:ContentCreatorInfo`
4. When enriched dicts are mapped into `TopContent` or `ContentByIdData`, Pydantic keeps only fields declared in the nested `user_info` model. Undeclared keys are not included in the final `model_dump()` output.

## 9. Fault Classification

| Possible cause | Is it the cause? | Explanation |
|---|---:|---|
| Database query does not fetch them | No | `GET_USERS_PROFILES` is `SELECT * FROM social_keyspace.user_profile WHERE user_id IN ?;`, so `created_at` and `country_code` are available when present in the row. |
| Repository ignores them | Partially no repository layer exists | The codebase does not have a separate repository class for this path. The service maps them but uses legacy names (`created_at`, `country`). |
| Service removes them | Not exactly | The service does not remove them; it names them differently from the requested contract. |
| Mapper skips them | Yes | The mapping to `UserInfo`/`ContentCreatorInfo` does not include `profile_created_at` or `country_code`. |
| Response schema does not contain them | Yes | `UserInfo` and `ContentCreatorInfo` omit both requested fields. |

Primary cause: response schema and mapper/service field-name mismatch.

## 10. Minimal Code Changes Required

Minimal fix:

1. Add `profile_created_at` and `country_code` to the nested user-info response models.
2. Populate those exact keys in `ContentService._fetch_user_map` and its fallback.
3. If the feed `_enrich_assets` path is part of the template/highlight response surface, extend `_fetch_user_details` and `UserInfo` there too.

No new Cassandra table, join, or service call is required for the `ContentService.enrich_content_items` path because it already calls `GET_USERS_PROFILES`.

## 11. Required Changes

### Change 1: Shared Content User Info Schema

| Item | Detail |
|---|---|
| Filename | `EU-Social/app/api/v1/schemas.py` |
| Class/function | `ContentCreatorInfo` |
| Current implementation | Defines `id`, `username`, `profile_picture`, `is_following`. |
| Proposed implementation | Add `profile_created_at: Optional[datetime] = None` and `country_code: Optional[str] = None`. |
| Explanation | Lets UGC/detail responses serialize the two requested fields inside `user_info`. |

Proposed snippet:

```python
class ContentCreatorInfo(BaseModel):
    id: UUID
    username: Optional[str] = None
    profile_picture: Optional[str] = None
    profile_created_at: Optional[datetime] = None
    country_code: Optional[str] = None
    is_following: bool = False
```

### Change 2: Feed/Static User Info Schema

| Item | Detail |
|---|---|
| Filename | `EU-Social/app/api/v1/feed/schemas.py` |
| Class/function | `UserInfo` |
| Current implementation | Defines `id`, `username`, `profile_picture`. |
| Proposed implementation | Add `profile_created_at: Optional[datetime] = None` and `country_code: Optional[str] = None`. |
| Explanation | Required for `TopContent.user_info` and `TrayAsset.user_info` because they use this model. |

Proposed snippet:

```python
class UserInfo(BaseModel):
    id: UUID
    username: Optional[str] = None
    profile_picture: Optional[str] = None
    profile_created_at: Optional[datetime] = None
    country_code: Optional[str] = None
```

### Change 3: Shared User Map

| Item | Detail |
|---|---|
| Filename | `EU-Social/app/api/v1/content/service.py` |
| Class/function | `ContentService._fetch_user_map` |
| Current implementation | Maps `country_code` to `country` and `created_at` to `created_at`. |
| Proposed implementation | Also map `country_code` and `profile_created_at` using the exact response keys. Keep legacy keys if current clients depend on them. |
| Explanation | This is the reusable implementation already used by static content listing and UGC content detail. |

Proposed snippet:

```python
user_map[user.user_id] = {
    "id": user.user_id,
    "username": user_name,
    "profile_picture": getattr(user, "image_url", None),
    "country": getattr(user, "country_code", None),
    "created_at": getattr(user, "created_at", None),
    "country_code": getattr(user, "country_code", None),
    "profile_created_at": getattr(user, "created_at", None),
}
```

Also update the fallback in `enrich_content_items`:

```python
{
    "id": uid,
    "username": "unknown",
    "profile_picture": None,
    "country": None,
    "created_at": None,
    "country_code": None,
    "profile_created_at": None,
}
```

### Change 4: Feed Helper User Map

| Item | Detail |
|---|---|
| Filename | `EU-Social/app/api/v1/feed/views.py` |
| Class/function | `_fetch_user_details`, `_enrich_assets` |
| Current implementation | `_fetch_user_details` returns only `username` and `profile_picture`; `_enrich_assets` passes only those fields to `UserInfo`. |
| Proposed implementation | Return and pass `profile_created_at` and `country_code`. |
| Explanation | Needed if Today's Highlights / Explore also use unified feed tray enrichment rather than only `get_content_listing`. |

Proposed `_fetch_user_details` addition:

```python
user_map[user.user_id] = {
    "username": user_name,
    "profile_picture": getattr(user, "image_url", None),
    "profile_created_at": getattr(user, "created_at", None),
    "country_code": getattr(user, "country_code", None),
}
```

Proposed `_enrich_assets` addition:

```python
user_info_obj = UserInfo(
    id=uid,
    username=u_info["username"],
    profile_picture=u_info["profile_picture"],
    profile_created_at=u_info.get("profile_created_at"),
    country_code=u_info.get("country_code"),
)
```

Fallback:

```python
{
    "username": DEFAULT_USERNAME,
    "profile_picture": None,
    "profile_created_at": None,
    "country_code": None,
}
```

## 12. Additional Joins, Queries, Or Service Calls

No additional join is needed for the main static/template content listing path. Cassandra data is already fetched from `social_keyspace.user_profile` through:

```sql
SELECT * FROM social_keyspace.user_profile WHERE user_id IN ?;
```

No new service call is required unless the missing public `/creator-economy/v1/templates/{template_id}` route lives in another service outside this local codebase. In that case, that gateway/service must apply the same schema and mapper fix.

## 13. Reusable Existing Implementation

Reusable implementation:

```text
EU-Social/app/api/v1/content/service.py
  ContentService.enrich_content_items
  ContentService._fetch_user_map
```

This helper already batches creator IDs, reads `user_profile`, and attaches `user_info`.

## 14. Original Data Source

| Output field | Original source | Local ingestion/sync |
|---|---|---|
| `profile_created_at` | `social_keyspace.user_profile.created_at` | `EU-Social/app/db/cassandra_utils.py:upsert_user_profile`, set from message timestamp when `event_type == "USER_CREATED"`. |
| `country_code` | `social_keyspace.user_profile.country_code` | `EU-Social/app/db/cassandra_utils.py:upsert_user_profile`, mapped from incoming `data["country"]`. |

`ensure_user_profile_table_exists` declares both Cassandra columns:

```sql
country_code text,
created_at timestamp,
```

## 15. Search Occurrences

### `profile_created_at`

No occurrences were found in `EU-Social`.

### `country_code`

| Path | Explanation |
|---|---|
| `EU-Social/app/core/constants.py` | Defines `COUNTRY_CODE = "country_code"`. |
| `EU-Social/app/db/cassandra_utils.py` | Creates `user_profile.country_code`; maps incoming `data["country"]` to `country_code`. |
| `EU-Social/app/api/v1/content/service.py` | Reads `user.country_code` in `_fetch_user_map`; reads it as `nationality` in `format_user_details`. |

### `user_info`

| Path | Explanation |
|---|---|
| `EU-Social/app/api/v1/content/service.py` | Creates and attaches `item["user_info"]` in `enrich_content_items`. |
| `EU-Social/app/api/v1/content/views.py` | Returns enriched content as `ContentByIdData` / grouped content; `user_info` is part of the model. |
| `EU-Social/app/api/v1/feed/views.py` | Creates `UserInfo` in `_enrich_assets`; maps `item.get("user_info")` into `TopContent` in `get_content_listing`. |
| `EU-Social/app/api/v1/feed/schemas.py` | Declares `user_info: Optional[UserInfo]` on `TrayAsset` and `TopContent`. |
| `EU-Social/app/api/v1/likes/views.py` | Builds a local `u_info` stub for liked content responses. |
| `EU-Social/app/api/v1/schemas.py` | Declares `user_info: ContentCreatorInfo` on `DetailedContentResponse` and `ContentByIdData`. |

## 16. Pydantic Models / DTOs / Serializers / Mappers

| File | Model/function | Role |
|---|---|---|
| `EU-Social/app/api/v1/feed/schemas.py` | `UserInfo` | Nested user object for feed/static content responses. |
| `EU-Social/app/api/v1/feed/schemas.py` | `TrayAsset` | Feed asset response model. |
| `EU-Social/app/api/v1/feed/schemas.py` | `TopContent` | Static/top content listing asset model. |
| `EU-Social/app/api/v1/feed/schemas.py` | `TopContentResponse` | Declared response wrapper model, not directly used by `get_content_listing`. |
| `EU-Social/app/api/v1/schemas.py` | `ContentCreatorInfo` | Nested user object for content detail/grouped content. |
| `EU-Social/app/api/v1/schemas.py` | `DetailedContentResponse` | Detailed content response model. |
| `EU-Social/app/api/v1/schemas.py` | `ContentByIdData` | Content-by-id response model. |
| `EU-Social/app/api/v1/content/service.py` | `format_content_data` | Maps Cassandra content rows into dicts. |
| `EU-Social/app/api/v1/content/service.py` | `_fetch_user_map` | Maps Cassandra profile rows into user-info dicts. |
| `EU-Social/app/api/v1/content/service.py` | `enrich_content_items` | Attaches `user_info`, counters, and viewer flags. |
| `EU-Social/app/utils/standard_response.py` | `standard_response` | Final JSON response wrapper using `jsonable_encoder`. |

## 17. Complete Call Graph

### Static/Template-Like Content Listing

```text
app/api/application.py:get_app
  -> app.include_router(api_router, prefix="/social/v1")
  -> app/api/v1/router.py:api_router.include_router(feed.router, prefix="/feed")
  -> app/api/v1/feed/views.py:get_content_listing
     -> validate_common_headers_optional_token
     -> get_cassandra_session
     -> get_redis_connection
     -> generate_cache_key
     -> get_cache_service
     -> cache_service.get
     -> session.prepare(CassandraQueries.GET_TOP_CONTENT_BY_LIST_NAME)
     -> session.execute(... name ...)
     -> ContentService.get_multiple_content_details
        -> session.prepare(CassandraQueries.GET_MULTIPLE_CONTENT_DETAILS)
        -> session.execute(... content_ids ...)
        -> ContentService.format_content_data(row)
     -> ContentService.enrich_content_items
        -> ContentService._fetch_user_map
           -> session.prepare(CassandraQueries.GET_USERS_PROFILES)
           -> session.execute(... creator_ids ...)
           -> build user_map
        -> ContentService._fetch_content_stats
           -> CassandraQueries.GET_CONTENT_COUNTERS
           -> CassandraQueries.GET_LIKE_COUNT
           -> CassandraQueries.GET_COMMENT_COUNT
        -> ContentService.fetch_social_flags
           -> CassandraQueries.GET_USER_LIKES
           -> CassandraQueries.GET_SAVED_CONTENTS
           -> CassandraQueries.GET_FOLLOWING
        -> ContentService.increment_view_counts_bulk
        -> item["user_info"] = user_info
     -> TopContent(...)
     -> c.model_dump()
     -> standard_response
        -> jsonable_encoder(data)
        -> build_meta
        -> JSONResponse
```

### Social UGC Content Detail

```text
app/api/application.py:get_app
  -> app.include_router(api_router, prefix="/social/v1")
  -> app/api/v1/router.py:api_router.include_router(content.router)
  -> app/api/v1/content/views.py:get_content
     -> validate_common_headers_optional_token
     -> get_cassandra_session
     -> get_redis_connection
     -> generate_cache_key
     -> get_cache_service
     -> cache_service.get
     -> ContentService.increment_view_count
     -> ContentService.get_content_details
        -> session.prepare(CassandraQueries.GET_CONTENT_DETAILS)
        -> session.execute(... content_id ...)
        -> ContentService.format_content_data(row)
     -> ContentService.enrich_content_items
        -> ContentService._fetch_user_map
        -> ContentService._fetch_content_stats
        -> ContentService.fetch_social_flags
        -> item["user_info"] = user_info
     -> ContentByIdData(**enriched_data[0])
     -> model_dump()
     -> standard_response
     -> JSONResponse
```

## Root Cause

The underlying user profile data exists in Cassandra and is fetched by the existing user-profile query. The bug is in the response contract layer: the service maps `created_at` and `country_code` to legacy/internal names (`created_at`, `country`), while the response models used for `user_info` do not declare the requested API fields `profile_created_at` and `country_code`. Pydantic therefore omits those keys from the final JSON.

## Recommended Fix

Add `profile_created_at` and `country_code` to the nested user-info models and populate those exact keys from `social_keyspace.user_profile.created_at` and `social_keyspace.user_profile.country_code` in the existing user map helpers. Reuse `ContentService._fetch_user_map`; no new database query or join is needed for the main static/template content listing path.
