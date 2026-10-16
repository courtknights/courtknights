# ADR-015: Partial Updates Use JSON Merge Patch (RFC 7396)

- **Status:** accepted
- **Date:** 2026-10-12
- **Deciders:** aagea

---

## Context

The first partial-update endpoint, `PUT /api/v1/users/me/profile`
(FEATURE_user_profile, #93), decodes optional fields into Go pointers. An
omitted field and an explicit `null` both decode to `nil`, so both mean "leave
unchanged". As a result:

- **A field can never be cleared.** Once a user sets `date_of_birth` or
  `gender`, they cannot withdraw it, which is a privacy problem (#121).
- **Nested objects are inconsistent.** `preferences` is replaced wholesale
  while top-level fields are merged, so a partial preferences update silently
  erases the keys it does not mention.
- **`PUT` is used for a partial update**, which contradicts its HTTP meaning
  (full replacement of the resource).

More partial-update endpoints are coming (leagues, teams, matches, club
settings). Without a rule, each would invent its own "clear" convention.

## Decision

Every endpoint that updates **part** of a resource follows these rules.

### 1. `PATCH` with JSON Merge Patch semantics

Partial updates use `PATCH` and [RFC 7396](https://www.rfc-editor.org/rfc/rfc7396):

| Request member | Effect |
|----------------|--------|
| omitted | unchanged |
| `null` | cleared (the field reads as `null` afterwards) |
| a value | set to that value |
| an object, for an object-valued field | merged recursively with the same three rules |

An empty object `{}` is valid and changes nothing.

### 2. Media types

Requests with `Content-Type: application/merge-patch+json` or
`application/json` are accepted. Media-type parameters such as `charset` are
ignored. Any other media type → `415 Unsupported Media Type`. The body must
be a JSON object; anything else → `400`.

### 3. Validation

- **`null` on a mandatory field → `400`.** Mandatory fields can be changed but
  never cleared.
- **Empty or whitespace-only strings → `400`.** `null` is the only way to
  clear a field; there is no second "empty" representation.
- **Unknown members → `400`**, at the top level and inside nested objects.
  A typo such as `"date_of_brith"` must never be silently ignored.
- A member with the wrong JSON type (e.g. an array for a string field) or a
  nested field that is not an object → `400`.
- A rejected request changes nothing.

### 4. `PUT` is reserved for full replacement

`PUT` is used only when the client sends the complete resource. It is never
used for partial updates.

### 5. Implementation pattern

- The handler decodes the body into `map[string]json.RawMessage` with the shared
  helper `api/common.DecodeMergePatch`. That way it can tell an omitted member
  from an explicit `null`, which plain struct decoding into pointers cannot.
- The domain represents each optional member as a tri-state value (unchanged /
  cleared / set), e.g. `profile.Field[T]`, never as a bare pointer.
- The repository writes `NULL` for cleared columns and merges object-valued
  columns (e.g. `jsonb`) in SQL, so concurrent partial updates of different
  keys do not overwrite each other.

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| JSON Patch (RFC 6902) — a list of `add` / `remove` / `replace` operations | More expressive than needed (array positions, `move`, `test`), much harder to validate field by field, and unfamiliar to most clients. Merge Patch covers every current case. |
| Keep `PUT`, add sentinel values (e.g. `""` or `"__clear__"`) | Overloads real values with special meanings, needs a sentinel per type (dates, enums), and keeps `PUT` for a partial update. |
| Per-field `clear_*` flags (`{"clear_city": true}`) | Doubles the request schema, can conflict with a value for the same field, and has no standard tooling. |
| `PUT` as true full replacement | Forces clients to read the whole resource before every change and races with concurrent writes; overkill for one-field edits. |
| Ignore unknown members (RFC 7396 default) | Typos silently succeed and the client believes the change was applied. |

## Consequences

### Positive

- Users can clear any optional field with a single, standard request.
- One rule for every partial update across the API; generic merge-patch
  tooling works for every client (Angular web app, CLI).
- Nested objects behave consistently with top-level fields.
- Strict validation (unknown members, blank strings) makes client mistakes
  visible immediately.

### Negative

- Handlers must decode member by member instead of binding into a struct; the
  shared `DecodeMergePatch` helper and the tri-state domain type keep this
  manageable.
- Existing `PUT` partial-update endpoints are breaking changes when migrated.
  The only one today, `PUT /users/me/profile`, has no client yet (#121).
- A client that sends a full object with `null` placeholders now clears those
  fields, where `PUT` used to ignore them.
- Merge Patch cannot set a field to a literal `null` value distinct from
  "cleared", and cannot edit arrays element by element. No current field needs
  either. A future need would require a new ADR (e.g. JSON Patch for that
  endpoint).

## References

- [RFC 7396 — JSON Merge Patch](https://www.rfc-editor.org/rfc/rfc7396)
- [RFC 5789 — PATCH Method for HTTP](https://www.rfc-editor.org/rfc/rfc5789)
- Issue [#121](https://github.com/courtknights/courtknights/issues/121) — clearable profile fields
- `docs/specs/FEATURE_clearable_profile_fields/02_architecture.md` — first application of this ADR
- [ADR-008](ADR-008_layered-architecture.md) — handler / manager / repository responsibilities
