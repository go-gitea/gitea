# Reputation labels

Gitea uses the `Badge` definition as a reusable, instance-managed label. A
label can describe an achievement for a user or an administrator's trust,
verification, or role assessment for a user, repository, or organization.

## Requirements and behavior

Site administrators create labels with a unique slug, an optional description,
a color, and an optional image URL. Administrators can assign and remove labels
from users, repositories, and organizations through the site administration
pages. Assignment is manual; automatic and rule-based assignment is not part of
this feature.

Labels are shown wherever the relevant entity is displayed. Repository and
organization labels can also be selected as filters on the corresponding Explore
pages. A filter selects entities that have the label; it does not hide other
labels assigned to a matching entity.

## Definitions and assignments

The `badge` table stores the reusable label definition. Assignments are kept in
separate mapping tables:

```text
badge
├── user_badge
├── repo_badge
└── org_badge
```

The mapping table determines the entity type. A repository assignment is not a
user assignment, and an organization assignment is not an assignment to an
individual member. Each mapping has a unique constraint so the same label
cannot be assigned twice to the same entity.

Deleting a label removes its assignments from all mapping tables in the same
transaction.

Organizations use Gitea's shared user record internally. Organization badge
loading therefore receives organization records represented as user objects,
but it always reads from `org_badge` and attaches only organization labels.

## Permissions and visibility

Only site administrators manage label definitions and assignments. End users
can see labels when they can see the associated user, repository, or
organization. This feature does not add a separate visibility configuration.

## Rendering

The shared badge template is reused by user, repository, organization, and list
views. Existing user badge behavior is preserved: user views may render an
image when `ImageURL` is set. Repository and organization views render compact
text labels with the configured color, even when the shared definition has an
image URL. Multiple labels are rendered in one inline list.

The shared definition is intentional: the assignment table supplies the entity
scope, while the rendering context supplies the presentation policy. The
current implementation does not introduce a separate `ReputationLabel` table.

## Loading and compatibility

List views load assignments in batches to avoid one database query per entity.
Existing user badge APIs and user badge assignments remain supported. Repository
and organization assignments use their own model methods, migrations, and
database mappings.

