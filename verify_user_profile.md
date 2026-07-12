# Verification: Template `fetch_user_details` And User Profile Fields

## Scope

Primary files checked:

- `EU_Creator_Economy/app/api/v1/template/views.py`
- `EU_Creator_Economy/app/api/queries.py`

Schema evidence for `social_keyspace.user_profile` was found in:

- `EU-Social/app/db/cassandra_utils.py`

No credentials, secrets, tokens, passwords, certificates, connection strings, or environment variable values are included.

## `fetch_user_details()` Complete Function

File: `EU_Creator_Economy/app/api/v1/template/views.py`

```python
def fetch_user_details(
    session: Session,
    user_ids: List[UUID],
) -> Dict[UUID, Dict[str, Any]]:
    """Fetch user details from Cassandra."""
    user_map: Dict[UUID, Dict[str, Any]] = {}
    if not user_ids:
        return user_map

    try:
        stmt = session.prepare(CreatorEconomyQueries.GET_USERS_PROFILES)
        users = session.execute(stmt, (user_ids,))
        for user in users:
            names = [
                n
                for n in [
                    (getattr(user, "firstname", None) or "").strip(),
                    (getattr(user, "lastname", None) or "").strip(),
                ]
                if n
            ]
            user_name = " ".join(names) if names else "unknown"

            user_map[user.user_id] = {
                "id": str(user.user_id),
                "username": user_name,
                "profile_picture": getattr(user, "image_url", None),
            }
        # Also add a string-keyed version for easier lookup if needed
        # but the current logic uses UUID keys for matching
    except Exception as e:
        logger.error(f"Error fetching user details: {e}")

    return user_map
```

## `GET_USERS_PROFILES` Definition

File: `EU_Creator_Economy/app/api/queries.py`

```python
GET_USERS_PROFILES = """
    SELECT * FROM social_keyspace.user_profile WHERE user_id IN ?;
"""
```

## Does The Row Contain `created_at` And `country_code`?

Based on local schema code, yes. `GET_USERS_PROFILES` uses `SELECT *` from `social_keyspace.user_profile`, and the shared Cassandra table schema includes both fields.

Schema source file: `EU-Social/app/db/cassandra_utils.py`

```python
CREATE TABLE IF NOT EXISTS {settings.cassandra_keyspace}.user_profile (
    user_id uuid PRIMARY KEY,
    about_me text,
    avatar_id int,
    birth_date date,
    country_code text,
    created_at timestamp,
    firstname text,
    gender text,
    image_url text,
    lastname text,
    modified_at timestamp,
    nick_name text
)
```

The same file maps incoming profile data into these Cassandra columns:

```python
mapped_data = {
    "user_id": UUID(data.get("uuid")) if data.get("uuid") else None,
    "about_me": data.get("about_me"),
    "avatar_id": (
        int(data.get("avatar_id")) if data.get("avatar_id") is not None else None
    ),
    "birth_date": birth_date_obj,
    "country_code": data.get("country"),
    "firstname": data.get("firstname"),
    "gender": data.get("gender"),
    "image_url": data.get("image"),
    "lastname": data.get("lastname"),
    "modified_at": timestamp_dt,
    "nick_name": data.get("nick_name"),
    "created_at": timestamp_dt if event_type == "USER_CREATED" else None,
}
```

## Row Object Fields Accessed In Code

`fetch_user_details()` currently accesses these returned row fields:

| Field | Access pattern | Used for |
|---|---|---|
| `user_id` | `user.user_id` | `user_map` key and returned `id`. |
| `firstname` | `getattr(user, "firstname", None)` | Builds `username`. |
| `lastname` | `getattr(user, "lastname", None)` | Builds `username`. |
| `image_url` | `getattr(user, "image_url", None)` | Returned as `profile_picture`. |

Fields available from the schema but not currently mapped in `fetch_user_details()`:

| Field | Expected API field |
|---|---|
| `created_at` | `profile_created_at` |
| `country_code` | `country_code` |

## Verification Summary

- `GET_USERS_PROFILES` fetches all columns from `social_keyspace.user_profile`.
- Local schema code for `social_keyspace.user_profile` includes `created_at` and `country_code`.
- The Cassandra row object should therefore expose those fields when the table is in sync with the local schema.
- The current `fetch_user_details()` function does not read or return either field, so they are lost when `user_info` is built.

Recommended mapping:

```python
user_map[user.user_id] = {
    "id": str(user.user_id),
    "username": user_name,
    "profile_picture": getattr(user, "image_url", None),
    "profile_created_at": getattr(user, "created_at", None),
    "country_code": getattr(user, "country_code", None),
}
```
