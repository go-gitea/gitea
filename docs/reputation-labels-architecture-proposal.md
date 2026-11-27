# Reputation labels architecture proposal

Status: **proposal for maintainer confirmation** (not yet implemented).  
Related: [#36334](https://github.com/go-gitea/gitea/issues/36334), [#39596](https://github.com/go-gitea/gitea/pull/39596), [review comment](https://github.com/go-gitea/gitea/pull/39596#issuecomment-7034884898).

This document distinguishes **verified facts** (current code and history) from **recommendations** (proposed direction) and **open questions** (maintainer decisions).

---

## 1. Problem statement

Instance administrators need a way to mark repositories and organizations as trusted, verified, or otherwise policy-approved, show those marks in the UI, and filter Explore by them ([#36334](https://github.com/go-gitea/gitea/issues/36334)).

Gitea already has **user badges** (definitions in `badge`, assignments in `user_badge`). Pull request [#39596](https://github.com/go-gitea/gitea/pull/39596) extends that definition table to repositories and organizations via `repo_badge` and `org_badge`, but **without an explicit scope**. Any badge definition can be assigned to any entity type, and Explore filters list every badge.

Maintainers disagree whether that is correct reuse of badge infrastructure or an abuse of an achievement-oriented feature. Silverwind asked for a **decision on “scope vs separate concept” before further architectural work**.

---

## 2. Existing architecture (verified)

### Upstream user badges

| Fact | Evidence |
|------|----------|
| Definitions live in `badge` (`Slug`, `Description`, `ImageURL`) | `models/user/badge.go` on `main`; migration `v224` |
| Assignments in `user_badge` with unique `(user_id, badge_id)` | migrations `v224`, `v329` |
| Original intent: award / achievement-style marks | [#20607](https://github.com/go-gitea/gitea/pull/20607) |
| Admin UI for create / edit / assign to users | [#36752](https://github.com/go-gitea/gitea/pull/36752) |
| Admin API for listing / adding / removing user badges | `GET/POST/DELETE /admin/users/{username}/badges` |
| User deletion cleans `user_badge` | `services/user/delete.go` |
| No scope field on `Badge` | Confirmed on `main` and on this PR |

### This PR’s extension

| Fact | Evidence |
|------|----------|
| Shared definition package `models/badges` with added `Color` | `models/badges/badge.go` |
| Mapping tables `user_badge`, `repo_badge`, `org_badge` | entity packages + migration `373` |
| Entity type implied only by which mapping table is used | Docs and model comments |
| Organizations are `user` rows (`UserTypeOrganization`) but org labels load from `org_badge`, not `user_badge` | `models/organization/badge.go` |
| Deleting a badge removes all mapping rows | `badges.DeleteBadge` |
| Explore filters repos via `repo_badge` and orgs via `org_badge` | `models/repo/repo_list.go`, `models/user/search.go` |

### Analogous Gitea patterns (verified)

| Pattern | How entity type is expressed | Relevance |
|---------|------------------------------|-----------|
| Topic + `RepoTopic` | Global definition + junction | Closest to Badge + mappings |
| Issue `Label` | `RepoID` / `OrgID` on the definition | Scope on the definition row |
| `Project.Type` | Explicit enum + FKs | Named scopes |
| PackageProperty | `RefType` + `RefID` | Polymorphic single table (weaker FKs) |

**Verified conclusion:** Keeping typed mapping tables is appropriate because organizations share the `user` primary-key space. A single polymorphic `(entity_type, entity_id)` assignment table would be harder to keep correct. The open question is only whether the **definition** needs an allowed-scope (or a separate table).

Behavioral docs for the current PR live in [`docs/reputation-labels.md`](reputation-labels.md).

---

## 3. Approach A — Generalize Badge with explicit scope

### Design (recommendation detail)

```text
badge
  slug, description, color, image_url
  allowed_scopes   // NEW: User | Repo | Org (multi-select)
├── user_badge
├── repo_badge
└── org_badge
```

- Existing badges backfill to **user scope only** (backward compatible).
- Admin create/edit sets allowed scopes.
- Assign endpoints and templates only list badges whose scope includes that entity.
- Explore repo filter lists Repo-scoped badges; org filter lists Org-scoped badges.
- Presentation stays contextual: users may show images; repo/org render text/color.
- Mapping tables remain (required for org vs user ID separation).

### Advantages

- Matches AdamMajer’s preference to generalize existing concepts.
- Directly answers silverwind’s “add a scope.”
- Reuses admin badge CRUD, search, slug validation, and colors.
- Restricts which definitions appear in pickers without inventing a second product surface.

### Disadvantages

- Still one physical “badge” concept — conceptual mixing is only partially solved (mitigate with UI naming for repo/org scopes).
- Needs a careful migration default and admin UX for scopes.
- Multi-scope definitions share one description/color/image (usually desirable).

### Impact summary

| Area | Change |
|------|--------|
| Database | Add `allowed_scopes` (or boolean flags); backfill existing → User |
| Models / services | Filter `SearchBadges` by scope; enforce on add-assignment |
| Admin UI | Scope controls on create/edit; assign pages filtered by scope |
| Explore | Scope-filtered dropdowns |
| Backward compatibility | Existing user badge behavior unchanged |
| Testing | Scope enforcement, Explore lists, migration backfill, reject cross-scope assign |

---

## 4. Approach B — Separate Reputation Label concept

### Design

```text
badge + user_badge                 // unchanged achievements
reputation_label                   // NEW definition
├── repo_reputation_label
└── org_reputation_label
```

### Advantages

- Cleanest split between achievements and reputation (addresses bircni most directly).
- Avoids “abusing” badge infrastructure (wxiaoguang).
- Independent evolution (fields, workflows, future automation).

### Disadvantages

- Near-duplicate definition CRUD, validation, admin templates, locale, and colors.
- Conflicts with AdamMajer’s preference to reuse existing concepts when possible.
- Two admin areas for similar chips; larger migration and review surface.
- Presentation differences do not require separate definition tables (already view policy).

### Impact summary

| Area | Change |
|------|--------|
| Database | New definition + two mapping tables; leave `badge` / `user_badge` alone |
| Models / services | Parallel packages and handlers |
| Admin UI | New section beside Badges |
| Explore | Filters join the new tables only |
| Backward compatibility | Strongest isolation for user badges |
| Testing | Full new suite plus user-badge regression coverage |

---

## 5. Comparison table

| Criterion | A: Scoped Badge | B: Separate ReputationLabel |
|-----------|-----------------|-----------------------------|
| AdamMajer (generalize) | Strong fit | Weak fit |
| bircni (don’t mix concepts) | Partial (scope + UI naming) | Strong fit |
| wxiaoguang (infra clarity) | Needs docs + enforcement | Strong fit |
| silverwind (decide scope vs separate) | Explicit scope | Explicit separate |
| Database complexity | Low | Higher |
| Code duplication | Lower | Higher |
| Org-as-user safety | Mapping tables retained | Same need for typed maps |
| Explore filters | Scope query | Separate catalog |
| Backward compatibility | Backfill User scope | Untouched user badges |
| Fit with current PR | Incremental | Rewrite of repo/org half |

---

## 6. Recommended architecture

**Recommendation: Approach A — one `badge` definition table, typed mapping tables, and an explicit allowed scope on each definition.**

Do **not** introduce a separate `ReputationLabel` table unless maintainers reject scoped badges.

### Why

1. Smallest change that unblocks silverwind’s open question.
2. Aligns with AdamMajer’s schema preference and lunny’s earlier suggestion to extend badges to organizations.
3. Addresses mixing concerns operationally: achievement-only badges stay user-scoped; trust labels opt into repo/org scopes.
4. Preserves the correct assignment model for org-as-user primary keys.

If maintainers prefer a hard conceptual split, fall back to Approach B.

### Maintainer feedback synthesis

| Maintainer | Position | How A responds |
|------------|----------|----------------|
| AdamMajer | Prefer generalizing existing concepts | Yes |
| bircni | Achievements ≠ reputation | Partial — scope + naming |
| wxiaoguang | Don’t abuse badge infra; document design | Scope + this document |
| silverwind | Decide scope vs separate before more design work | Explicit scope choice |
| lunny | Extending badges to orgs is reasonable | Yes |

There is **no verified consensus** that Approach B is required. This recommendation prioritizes A; B remains the fallback.

---

## 7. Migration and backward compatibility

Under Approach A (if approved):

1. Add scope storage on `badge` (bitfield or columns).
2. Backfill every existing row to **user-only**.
3. New repo/org reputation labels are created with Repo and/or Org scopes as chosen by admins.
4. Reject assignments that violate allowed scopes.
5. Keep `user_badge` / `repo_badge` / `org_badge` and public user-badge APIs.

User profile image badges, admin user assignment, and existing slugs continue to work. Repo/org labels already introduced by this PR remain; only their **eligibility** in pickers/filters becomes scope-aware after the follow-up change.

---

## 8. Unresolved maintainer questions

1. Confirm **Approach A (scoped Badge)** vs **Approach B (separate ReputationLabel)**?
2. Under A: may one definition allow **multiple scopes** (for example Repo+Org), or must scopes be exclusive?
3. Admin UI naming: keep “Badges” with scopes, or surface “Reputation labels” for non-user scopes?
4. Images: keep image badges for **users only**, deprecate images, or allow images for repo/org too?
5. Should Explore **users** also get a badge filter (out of the original #36334 scope)?
