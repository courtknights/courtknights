# FEATURE_clubs — Business Context

- **Status:** in-review
- **Feature ID:** FEATURE_clubs
- **Issue:** [#157](https://github.com/courtknights/courtknights/issues/157)
- **Author:** aagea (scope decisions) — drafted by the AI agent
- **Date:** 2026-10-20

---

## Problem statement

CourtKnights is built to let clubs and communities run their own padel
competitions. The domain model (`docs/context/glossary.md`,
`docs/context/users.md`) places the **club** at the top: every competition
belongs to a club, and the people who manage competitions get their authority
from the club.

The platform does not have clubs yet. A user is only a global `user` or a
global `admin`. Without clubs:

- there is nothing for a league to belong to (#158 depends on this feature);
- there is no way to say who may create or manage a competition, short of
  making them a platform administrator;
- a community cannot gather its players in one place.

## Users affected

See `docs/context/users.md`.

- **Any authenticated user:** creates a club, browses clubs and their members.
- **Club Admin:** edits the club and manages its members and admins.
- **Member:** belongs to a club. Being a member is what will later allow a
  user to take part in the club's competitions.
- **Platform admin** (global `admin` role): acts as a superuser in every club,
  for moderation and support.

The competition-level roles (Competition Organiser, Team Captain, Player) are
not part of this feature. They are introduced with the competitions that use
them (#158 and later).

## Business value

- Unblocks every competition feature: leagues, seasons, registrations,
  matches and statistics all hang off a club.
- Lets communities self-organise without asking a platform administrator:
  anyone can start a club and run it.
- Introduces scoped permissions (roles that apply inside one club), which every
  later feature reuses.

If we do not build it, leagues would have to be owned by a user or by the
platform, which contradicts the domain model and would require a costly data
migration later.

## Scope

### In scope

**Club**

- Any authenticated user can create a club. The creator becomes its first
  Club Admin automatically.
- A club has:
  - a **name** — required, not blank, unique across the platform (compared
    case-insensitively);
  - a **description** — optional free text.
- Club Admins can update the name and description, and clear the description.
  Updates follow ADR-015 (`PATCH` with JSON Merge Patch).
- Any authenticated user can list clubs (paginated) and view a single club.

**Membership**

- Two club-level roles: **Club Admin** and **member**. A user has at most one
  role in a given club. Every Club Admin is also a member.
- A user can belong to any number of clubs, with a different role in each.
- A Club Admin adds an existing platform user to the club by user ID. The user
  joins as a member.
- A Club Admin removes a member from the club.
- A Club Admin promotes a member to Club Admin, or demotes a Club Admin to
  member.
- A club always keeps at least one Club Admin: removing or demoting the last
  one is rejected.
- A member can leave a club on their own, subject to the same rule.
- Any authenticated user can list a club's members with their role and basic
  profile data (display name).
- A user can list the clubs they belong to.

**Platform admin**

- A user with the global `admin` role can perform every Club Admin action in
  any club, without being a member of it.

### Out of scope

- Join requests, invitations by email, or open clubs that anyone can join.
- Courts and facilities. They come with match scheduling.
- Club deletion, archiving or ownership transfer (beyond promoting another
  Club Admin).
- Club logo, contact details, address or location.
- Private clubs or any per-club visibility setting: all clubs and their member
  lists are visible to every authenticated user.
- Competitions of any kind (#158).
- Frontend screens. This feature delivers the API only.

## User stories

### US-01 — Create a club

> As a signed-in user, I want to create a club so that my community has a
> place to organise its competitions.

The creator becomes the club's first Club Admin. A blank or already used name
is rejected.

### US-02 — Edit my club

> As a Club Admin, I want to change my club's name and description so that
> they stay accurate.

### US-03 — Add a player to my club

> As a Club Admin, I want to add a registered user to my club so that they can
> take part in its competitions later.

### US-04 — Share the administration

> As a Club Admin, I want to promote another member to Club Admin, or demote
> one, so that the work of running the club can be shared.

The last Club Admin cannot be demoted or removed, and cannot leave.

### US-05 — Remove a member or leave a club

> As a Club Admin, I want to remove a member who no longer belongs to the
> club. As a member, I want to leave a club I no longer play in.

### US-06 — Discover clubs and their members

> As a signed-in user, I want to browse clubs and see who belongs to them, so
> that I can find my community.

### US-07 — See my clubs

> As a signed-in user, I want to see the clubs I belong to and my role in
> each.

### US-08 — Moderate as platform admin

> As a platform admin, I want to manage any club as if I were its Club Admin,
> so that I can fix problems and support communities.

## Assumptions and dependencies

- **FEATURE_authentication:** users, global roles and the JWT middleware.
- **FEATURE_user_profile:** member listings show the member's `display_name`.
- **ADR-008:** club logic lives in its own Manager and Repository.
- **ADR-013:** the clubs feature reads users through a narrow interface,
  without depending on the auth feature's internals.
- **ADR-015:** club updates use `PATCH` with JSON Merge Patch.
- Adding a member requires knowing the user's ID. Users can be found through
  `GET /api/v1/users`.

## Open questions for review

1. **Name uniqueness.** This draft makes club names unique across the
   platform, ignoring case. The alternative is to allow duplicate names and
   tell clubs apart only by ID.
2. **Leaving a club.** This draft lets a member leave on their own. The
   alternative is that only a Club Admin can remove members.

## Glossary

Terms already defined in `docs/context/glossary.md`: *Club*, *Club Admin*,
*Member*. This feature adds:

| Term | Definition |
|------|-----------|
| Club role | The role a user holds inside one club: `admin` (Club Admin) or `member`. Independent of the user's global role. |
| Platform admin | A user with the global `admin` role. Acts as a Club Admin in every club. |
