# Domain Glossary

- **Last updated:** 2026-03-17

Terms are listed alphabetically. Add new terms here before using them in specs.

---

## C

**Club**
The top-level organisational unit. A club owns courts, runs competitions, and manages its members. A club has at least one Club Admin.

**Club Admin**
The user responsible for managing a club's resources, members, and overall configuration. See `docs/context/users.md`.

**Competition**
A generic term for any organised event — either a league, a knockout tournament, or a group stage + knockout. A competition always belongs to a club and has a defined format.

**Competition Organiser**
The user responsible for running a specific competition: configuring it, validating results, and resolving incidents. See `docs/context/users.md`.

**Court**
A physical padel court managed by a club. Courts are a resource that can be assigned to matches.

---

## F

**Format**
The structure that defines how a competition is played. CourtKnights supports three formats:
- **League (round-robin):** every pair or team plays against every other. Final standings are determined by points.
- **Knockout (elimination):** single-elimination bracket. Losers are eliminated; the last pair or team standing wins.
- **Group stage + knockout:** pairs or teams are divided into groups for a round-robin phase; top finishers advance to a knockout phase.

---

## G

**Group**
A subdivision within the group stage phase of a competition. Each group runs an internal round-robin to determine which participants advance to the knockout phase.

---

## L

**League**
Refers to both (1) the round-robin competition format and (2) colloquially to any competition organised on the platform.

---

## M

**Match**
A single game between two pairs or two teams within a competition. A match has a result (sets, games) and a date.

**Member**
A user who belongs to a club. Members can participate in competitions run by that club.

---

## P

**Padel**
The racket sport this platform is built for. Played in pairs (2 vs 2) on an enclosed court. CourtKnights is designed primarily for padel; support for other sports may be added in future versions.

**Pair**
The unit of competition in pair-based competitions. A pair consists of exactly two players. Either player can perform administrative actions for the pair.

**Player**
An individual user participating in a competition. See `docs/context/users.md`.

---

## R

**Result**
The outcome of a match, expressed in sets and games (e.g. 6-4, 6-3). Results are recorded by participants and validated by the Competition Organiser.

**Round**
A stage within a competition where a set of matches is played. In a league, a round groups all matches played in a given week or date. In a knockout, a round is one elimination stage (e.g. quarter-finals).

---

## S

**Season**
A time-bounded edition of a competition. A club may run multiple seasons of the same league over time.

**Standing**
The ranked position of a pair or team within a competition at any point in time, calculated from match results.

**Statistics**
Aggregated performance data for a player or pair: number of matches played, wins, losses, win percentage, sets won, games won, and match history.

---

## T

**Team**
The unit of competition in team-based competitions. A team consists of multiple players and has one or more captains.

**Team Captain**
A player within a team who has administrative responsibilities for that team. See `docs/context/users.md`.

**Tournament**
Used interchangeably with *Competition* in everyday language. In the codebase and specs, prefer *Competition* for precision.
