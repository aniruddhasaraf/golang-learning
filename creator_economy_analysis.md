# Creator Economy Template `user_info` Field Investigation

## Scope And Safety

Repository analyzed: `EU_Creator_Economy` only.

No credentials, secrets, tokens, passwords, certificates, connection strings, environment variable values, or sensitive configuration values are included in this document.

Investigated API:

```http
GET /creator-economy/v1/templates/{template_id}
```

Problem:

`user_info` in non-UGC/template responses is missing:

- `profile_created_at`
- `country_code`

## 1. FastAPI Route

The route exists in this repository.

| Layer | File | Implementation |
|---|---|---|
| App prefix | `EU_Creator_Economy/app/api/application.py` | `app.include_router(router=api_router, prefix="/creator-economy/v1")` |
| Template router mount | `EU_Creator_Economy/app/api/v1/router.py` | `api_router.include_router(template_views.router, prefix="/templates", tags=["Templates"])` |
| Route handler | `EU_Creator_Economy/app/api/v1/template/views.py` | `@router.get("/{template_id}")` |
| Handler function | `EU_Creator_Economy/app/api/v1/template/views.py` | `get_template_by_id(...)` |

Resolved route:

```text
/creator-economy/v1 + /templates + /{template_id}
= GET /creator-economy/v1/templates/{template_id}
```

## 2. Complete Execution Flow

```text
HTTP GET /creator-economy/v1/templates/{template_id}
  -> app/api/application.py:get_app()
     -> app.include_router(api_router, prefix="/creator-economy/v1")
  -> app/api/v1/router.py
     -> include template_views.router at /templates
  -> app/api/v1/template/views.py:get_template_by_id()
     -> validate_common_headers_optional_token
     -> get_redis_connection
     -> get_cassandra_session
     -> build_cache_key(CacheKeyTemplates.TEMPLATES_DETAIL, ...)
     -> increment_template_view_count()
        -> CreatorEconomyQueries.INCREMENT_TEMPLATE_VIEW_COUNT
        -> creator_keyspace.template_counter
     -> get_cache(...)
        -> if cache hit:
           -> _refresh_cached_detail_counters()
              -> fetch_template_counters()
              -> fetch_template_interactions()
           -> standard_response(...)
           -> JSONResponse
        -> if cache miss:
           -> execute_and_transform_cassandra()
              -> CreatorEconomyQueries.GET_TEMPLATE_BY_ID
              -> creator_keyspace.template
              -> TemplateResponse.model_validate(...).model_dump(mode="json")
           -> fetch_user_details()
              -> CreatorEconomyQueries.GET_USERS_PROFILES
              -> social_keyspace.user_profile
              -> build user_info dict
           -> fetch_template_counters()
              -> creator_keyspace.like_counter
              -> creator_keyspace.comments_by_template
              -> creator_keyspace.template_counter
           -> fetch_template_interactions()
              -> creator_keyspace.likes_by_template
              -> social_keyspace.saved_content_by_user
              -> social_keyspace.followers_by_user
           -> translate_rows(...)
           -> set_cache(...)
           -> standard_response(...)
           -> JSONResponse
```

## 3. Files Participating In The Response

| File | Role |
|---|---|
| `EU_Creator_Economy/app/api/application.py` | Creates FastAPI app and mounts API prefix. |
| `EU_Creator_Economy/app/api/v1/router.py` | Mounts template router under `/templates`. |
| `EU_Creator_Economy/app/api/v1/template/views.py` | Main route handler, enrichment helpers, cache refresh, user_info creation. |
| `EU_Creator_Economy/app/api/v1/schemas.py` | Pydantic response models: `TemplateResponse`, `UserInfo`. |
| `EU_Creator_Economy/app/api/queries.py` | Cassandra and Postgres query constants. |
| `EU_Creator_Economy/app/db/utils.py` | `execute_and_transform_cassandra`, which validates Cassandra rows into Pydantic dicts. |
| `EU_Creator_Economy/app/cache/base.py` | Cache key/get/set helpers used by the detail endpoint. |
| `EU_Creator_Economy/app/cache/dependencies.py` | Redis dependency. |
| `EU_Creator_Economy/app/db/cassandra_client.py` | Cassandra dependency. |
| `EU_Creator_Economy/app/utils/standard_response.py` | Final response wrapper. |
| `EU_Creator_Economy/app/utils/translate.py` | Translates `title` and `description` before caching/response. |
| `EU_Creator_Economy/app/utils/validate_headers.py` | Header validation dependency. |

Related list APIs that reuse the same `user_info` helper:

| File | Route / function | Why relevant |
|---|---|---|
| `EU_Creator_Economy/app/api/v1/template/views.py` | `get_all_templates` | Also adds `user_info` via `fetch_user_details`. |
| `EU_Creator_Economy/app/api/v1/playlist/views.py` | `get_templates_by_playlist` | Also adds `user_info` via `fetch_user_details`. |

## 4. Where `user_info` Is Created

`user_info` is created in:

```text
EU_Creator_Economy/app/api/v1/template/views.py
  fetch_user_details()
```

Current implementation:

```python
user_map[user.user_id] = {
    "id": str(user.user_id),
    "username": user_name,
    "profile_picture": getattr(user, "image_url", None),
}
```

It is attached in the detail endpoint:

```python
if author_id and author_id in user_map:
    template["user_info"] = user_map[author_id]
```

The same `fetch_user_details()` helper is reused by:

- `get_template_by_id`
- `get_all_templates`
- `get_templates_by_playlist`

## 5. Source For `profile_created_at` And `country_code`

These fields should come from the creator profile row in:

```text
social_keyspace.user_profile
```

The existing query is:

```python
GET_USERS_PROFILES = """
    SELECT * FROM social_keyspace.user_profile WHERE user_id IN ?;
"""
```

Expected mapping:

| API field | Cassandra field |
|---|---|
| `profile_created_at` | `social_keyspace.user_profile.created_at` |
| `country_code` | `social_keyspace.user_profile.country_code` |

Within this repository, `country_code` does not appear in the user-info mapping. `profile_created_at` does not appear anywhere.

## 6. Do These Values Already Exist In The Database Query?

For creator profile enrichment, yes, if the `social_keyspace.user_profile` table contains these columns.

`fetch_user_details()` uses `CreatorEconomyQueries.GET_USERS_PROFILES`, which is `SELECT *`. That means `created_at` and `country_code` are included in the Cassandra row when present in the table.

The initial template query does not include these fields, and it should not need to. `creator_keyspace.template` contains template data, while creator profile metadata belongs to `social_keyspace.user_profile`.

## 7. If Not In The Template Query, Where Should They Be Fetched?

They should be fetched in the existing creator profile enrichment step:

```text
fetch_user_details()
  -> CreatorEconomyQueries.GET_USERS_PROFILES
  -> social_keyspace.user_profile
```

No additional join or new service call is required. The repository already performs the separate profile lookup after reading the template row.

## 8. Where The Values Are Lost

The values disappear in `fetch_user_details()`.

The profile query returns full user profile rows via `SELECT *`, but the mapper keeps only:

- `id`
- `username`
- `profile_picture`

It ignores:

- `created_at`
- `country_code`

Additionally, the nested response model `UserInfo` does not define:

- `profile_created_at`
- `country_code`

So even if those keys were passed through Pydantic validation in `TemplateResponse`, they would need to be part of `UserInfo` to be part of the contract.

Important cache edge:

On cache hits, `get_template_by_id()` calls `_refresh_cached_detail_counters()` only. That refreshes counters/interactions, not `user_info`. Old cached templates may continue missing the new fields until cache expiry or explicit invalidation/versioning.

## 9. Comparison With Social UGC / Shared Utilities

Within `EU_Creator_Economy`, there is no imported Social UGC response mapper or shared UGC utility. The service queries Social-owned Cassandra tables directly:

- `social_keyspace.user_profile`
- `social_keyspace.saved_content_by_user`
- `social_keyspace.followers_by_user`

The closest shared contract is conceptual rather than code-level: both Social UGC and Creator Economy template responses should enrich content with creator profile data from `social_keyspace.user_profile`.

In this repository, the shared local utility for template/list/playlist responses is:

```text
EU_Creator_Economy/app/api/v1/template/views.py:fetch_user_details
```

Fixing this helper updates all Creator Economy template response surfaces that call it.

## 10. Exact Code Changes Required

### Change 1: Add Fields To `UserInfo`

| Item | Detail |
|---|---|
| Filename | `EU_Creator_Economy/app/api/v1/schemas.py` |
| Class/function | `UserInfo` |
| Current implementation | Contains only `id`, `username`, `profile_picture`. |
| Proposed implementation | Add `profile_created_at` and `country_code`. |
| Explanation | Documents and validates the nested `user_info` response contract. |

Current:

```python
class UserInfo(BaseModel):
    """User summary info."""

    id: UUID
    username: Optional[str] = None
    profile_picture: Optional[str] = None

    model_config = ConfigDict(from_attributes=True)
```

Proposed:

```python
class UserInfo(BaseModel):
    """User summary info."""

    id: UUID
    username: Optional[str] = None
    profile_picture: Optional[str] = None
    profile_created_at: Optional[datetime] = None
    country_code: Optional[str] = None

    model_config = ConfigDict(from_attributes=True)
```

`datetime` is already imported in this file.

### Change 2: Populate Fields In `fetch_user_details`

| Item | Detail |
|---|---|
| Filename | `EU_Creator_Economy/app/api/v1/template/views.py` |
| Class/function | `fetch_user_details` |
| Current implementation | Reads user rows but maps only `id`, `username`, `profile_picture`. |
| Proposed implementation | Also map `profile_created_at` from `user.created_at` and `country_code` from `user.country_code`. |
| Explanation | This is where the values are currently lost. |

Current:

```python
user_map[user.user_id] = {
    "id": str(user.user_id),
    "username": user_name,
    "profile_picture": getattr(user, "image_url", None),
}
```

Proposed:

```python
user_map[user.user_id] = {
    "id": str(user.user_id),
    "username": user_name,
    "profile_picture": getattr(user, "image_url", None),
    "profile_created_at": getattr(user, "created_at", None),
    "country_code": getattr(user, "country_code", None),
}
```

### Change 3: Handle Existing Cached Detail Responses

| Item | Detail |
|---|---|
| Filename | `EU_Creator_Economy/app/api/v1/template/views.py` or deployment/cache config |
| Class/function | `_refresh_cached_detail_counters` / cache key versioning |
| Current implementation | On cache hit, refreshes counters/interactions only. |
| Proposed implementation | Either invalidate `templates:detail:*` and list/playlist template caches, bump `settings.api_version`, or refresh missing `user_info` fields on cache hit. |
| Explanation | Without cache invalidation/versioning, old cached JSON can keep missing fields after the code fix. |

Minimal operational fix:

```text
Invalidate template detail/list/playlist caches or bump api_version after deployment.
```

Optional code-level cache refresh:

```python
if not (cached_template.get("user_info") or {}).get("profile_created_at"):
    if isinstance(author_id, UUID):
        user_map = fetch_user_details(session, [author_id])
        if author_id in user_map:
            cached_template["user_info"] = user_map[author_id]
```

Apply the same consideration to list caches returned by:

- `get_all_templates`
- `get_templates_by_playlist`

## 11. Database / Query Changes

No Cassandra query change is required for the detail endpoint if `social_keyspace.user_profile` already has `created_at` and `country_code`.

Existing query:

```python
GET_USERS_PROFILES = """
    SELECT * FROM social_keyspace.user_profile WHERE user_id IN ?;
"""
```

If the production table did not have these columns, they would need to be added/populated in the Social user profile sync. That is outside the local Creator Economy route code, and this repository already assumes the table exists.

## 12. Search Results

Search was performed in `EU_Creator_Economy` excluding generated static docs assets.

### `templates`

Relevant occurrences:

| Path | Explanation |
|---|---|
| `app/api/v1/router.py` | Mounts template router at `/templates`. |
| `app/api/v1/template/views.py` | Main templates list/detail/share handlers and cache refresh logic. |
| `app/api/v1/playlist/views.py` | Playlist API returns `"templates"` list and uses the same user enrichment helper. |
| `app/core/constants.py` | Cache keys/messages/descriptions for template APIs. |
| `app/utils/meilisearch_client.py` | Refers to templates index helper. |
| `app/utils/debezium_consumer.py` | Re-indexes template documents for search. |
| `app/core/exceptions/exceptions.py` | Template-not-found exception class/docstring. |
| `app/api/v1/template/services.py` | Builds share links under `/creator-economy/v1/templates/...`. |

### `template_id`

Relevant occurrences:

| Path | Explanation |
|---|---|
| `app/api/queries.py` | Template CQL/Postgres query constants, counters, likes, comments, short links. |
| `app/api/v1/template/views.py` | Route parameter, counters, interactions, cache keys, share routes. |
| `app/api/v1/template/services.py` | Template detail/share/short-link helper methods. |
| `app/api/v1/playlist/views.py` | Reads playlist template IDs from Postgres and fetches templates from Cassandra. |
| `app/api/v1/comments/views.py` | Comment APIs operate on template IDs. |
| `app/api/v1/like/views.py` | Like APIs operate on template IDs. |
| `app/api/v1/schemas.py` | Request/response schema fields. |
| `app/utils/kafka_consumer.py` | Template sync/delete handling. |
| `app/utils/debezium_consumer.py` | Meilisearch re-indexing and relation handling. |
| `app/utils/meilisearch_helpers.py` | Builds enriched search documents and relationship fields. |
| `app/core/constants.py` | Cache key template for detail API. |

### `user_info`

Every occurrence found:

| Path | Explanation |
|---|---|
| `app/api/v1/schemas.py:49` | `TemplateResponse.user_info: Optional[UserInfo]`. |
| `app/api/v1/template/views.py:330` | Reads `user_info` to infer author IDs from cached/list payloads. |
| `app/api/v1/template/views.py:583` | Assigns `user_info` in Meilisearch list path. |
| `app/api/v1/template/views.py:711` | Assigns `user_info` in Cassandra list fallback path. |
| `app/api/v1/template/views.py:857` | Assigns `user_info` in `get_template_by_id`. |
| `app/api/v1/playlist/views.py:187` | Assigns `user_info` in playlist template responses. |

### `profile_created_at`

No occurrences found in `EU_Creator_Economy`.

### `country_code`

Only local variable occurrences were found in filtering logic:

| Path | Explanation |
|---|---|
| `app/api/v1/template/views.py:516` | Local variable `country_codes` for Meilisearch country filter. |
| `app/api/v1/template/views.py:517` | Checks country filter values. |
| `app/api/v1/template/views.py:519` | Builds Meilisearch filter on `country.short_code`. |

No `user_info.country_code` mapping exists.

## 13. Complete Call Graph

```text
GET /creator-economy/v1/templates/{template_id}

app/api/application.py:get_app
  -> FastAPI(...)
  -> app.include_router(api_router, prefix="/creator-economy/v1")

app/api/v1/router.py
  -> api_router.include_router(template_views.router, prefix="/templates")

app/api/v1/template/views.py:get_template_by_id
  -> Depends(get_redis_connection)
  -> Depends(get_cassandra_session)
  -> Depends(validate_common_headers_optional_token)
  -> build_cache_key(CacheKeyTemplates.TEMPLATES_DETAIL, ...)
  -> increment_template_view_count(cassandra_session, template_id)
     -> session.prepare(CreatorEconomyQueries.INCREMENT_TEMPLATE_VIEW_COUNT)
     -> session.execute(... template_id ...)
  -> get_cache(cache_session, cache_key)
     -> cache hit:
        -> _refresh_cached_detail_counters(cassandra_session, cached_data, template_id, viewer_id)
           -> fetch_template_counters(cassandra_session, template_id)
              -> CreatorEconomyQueries.GET_LIKE_COUNT
              -> CreatorEconomyQueries.GET_COMMENT_COUNT
              -> CreatorEconomyQueries.GET_TEMPLATE_SHARE_VIEW_COUNT
           -> fetch_template_interactions(cassandra_session, template_id, author_id, viewer_id)
              -> CreatorEconomyQueries.CHECK_LIKE
              -> CreatorEconomyQueries.CHECK_SAVED_CONTENT
              -> CreatorEconomyQueries.CHECK_FOLLOWER
        -> standard_response(SuccessMessages.TEMPLATE_FETCHED, request, cached_data)
           -> jsonable_encoder(data)
           -> build_meta(...)
           -> JSONResponse
     -> cache miss:
        -> execute_and_transform_cassandra(
              cassandra_session,
              CreatorEconomyQueries.GET_TEMPLATE_BY_ID,
              TemplateResponse,
              (template_id,),
           )
           -> session.prepare(CreatorEconomyQueries.GET_TEMPLATE_BY_ID)
           -> session.execute(... template_id ...)
           -> row._asdict()
           -> TemplateResponse.model_validate(row_dict)
           -> model_dump(mode="json")
        -> template = template_data[0]
        -> author_id = UUID(template["user_id"])
        -> fetch_user_details(cassandra_session, [author_id])
           -> session.prepare(CreatorEconomyQueries.GET_USERS_PROFILES)
           -> session.execute(... [author_id] ...)
           -> map profile row into user_map[author_id]
        -> template["user_info"] = user_map[author_id]
        -> fetch_template_counters(cassandra_session, template_id)
        -> template.update(counters)
        -> fetch_template_interactions(cassandra_session, template_id, author_id, viewer_id)
        -> template.update(interactions)
        -> translate_rows([template], language, fields=["title", "description"])
        -> set_cache(cache_session, cache_key, template, ttl=CacheTTL.TTL_MAX)
        -> standard_response(SuccessMessages.TEMPLATE_FETCHED, request, template)
           -> jsonable_encoder(data)
           -> build_meta(...)
           -> JSONResponse
```

## Root Cause

`profile_created_at` and `country_code` are not fetched from the template table because they are creator profile fields, not template fields. The endpoint already performs the correct separate profile lookup with `GET_USERS_PROFILES`, but `fetch_user_details()` drops those fields when building `user_info`. The `UserInfo` Pydantic model also does not declare them.

## Recommended Fix

Add `profile_created_at` and `country_code` to `UserInfo`, and populate them in `fetch_user_details()` from `social_keyspace.user_profile.created_at` and `social_keyspace.user_profile.country_code`. Invalidate/bump template caches so previously cached responses do not keep returning the old `user_info` shape.
