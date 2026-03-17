# User Types and Roles

- **Last updated:** 2026-03-17

---

## Overview

CourtKnights has two layers of roles: **club-level** roles that manage the organisation and its resources, and **competition-level** roles that operate within a specific league or tournament.

---

## Club-level roles

### Club Admin

The owner and administrator of a club on the platform.

- Creates and configures the club
- Manages physical resources (courts, facilities)
- Handles logistical incidents (court availability, scheduling conflicts)
- Manages club membership — who can join and with what role
- Has full access to all competitions within the club

**One or more per club.**

---

## Competition-level roles

### Competition Organiser

Responsible for running a specific league or tournament within a club.

- Creates and configures the competition (format, rules, dates)
- Validates and approves match results
- Resolves disputes and incidents between participants
- Manages the competition calendar and scheduling
- Can be the same person as the Club Admin or a delegated member

**One or more per competition.**

### Team Captain

Applies to **team-based competitions** only.

- Represents a team within a competition
- Manages team membership (adds/removes players)
- Confirms match results on behalf of the team
- A team can have one or more captains

**One or more per team, in team-based competitions only.**

### Player

Any individual registered in a competition, either as part of a pair or a team.

- Participates in matches
- Views their own statistics and match history
- In **pair-based competitions**: either player in the pair can perform administrative actions for that pair (no designated captain role)
- In **team-based competitions**: acts under the team captain's authority

**Many per competition.**

---

## Competition formats and role applicability

| Role                  | Pair-based competition                 | Team-based competition |
| --------------------- | -------------------------------------- | ---------------------- |
| Club Admin            | ✓                                      | ✓                      |
| Competition Organiser | ✓                                      | ✓                      |
| Team Captain          | —                                      | ✓                      |
| Player                | ✓ (any player in pair acts as captain) | ✓                      |

---

## Permission summary

| Action                      | Club Admin | Competition Organiser | Team Captain | Player |
| --------------------------- | ---------- | --------------------- | ------------ | ------ |
| Create competition          | ✓          | —                     | —            | —      |
| Configure competition rules | ✓          | ✓                     | —            | —      |
| Validate match results      | ✓          | ✓                     | —            | —      |
| Resolve incidents           | ✓          | ✓                     | —            | —      |
| Manage team members         | ✓          | ✓                     | ✓            | —      |
| Confirm match result (pair) | ✓          | ✓                     | —            | ✓      |
| View statistics             | ✓          | ✓                     | ✓            | ✓      |
| Manage court resources      | ✓          | —                     | —            | —      |
