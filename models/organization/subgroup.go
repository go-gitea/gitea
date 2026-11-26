// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type subgroupHierarchyLockCtxKey struct{}

var subgroupHierarchyMu sync.Mutex

func acquireSubgroupHierarchyTxLock(ctx context.Context) error {
	switch {
	case setting.Database.Type.IsPostgreSQL():
		if _, err := db.GetEngine(ctx).Query("SELECT pg_advisory_xact_lock(1872001872)"); err != nil {
			return err
		}
		var locked []*Organization
		return db.GetEngine(ctx).
			Where(builder.Eq{"`type`": user_model.UserTypeOrganization}).
			Asc("id").
			ForUpdate().
			Cols("id").
			Find(&locked)
	case setting.Database.Type.IsMySQL():
		var locked []*Organization
		return db.GetEngine(ctx).
			Where(builder.Eq{"`type`": user_model.UserTypeOrganization}).
			Asc("id").
			ForUpdate().
			Cols("id").
			Find(&locked)
	case setting.Database.Type.IsMSSQL():
		_, err := db.GetEngine(ctx).Query("SELECT [id] FROM [user] WITH (UPDLOCK, HOLDLOCK) WHERE [type] = ? ORDER BY [id] ASC", user_model.UserTypeOrganization)
		return err
	default:
		return nil
	}
}

// WithSubgroupHierarchyLock serializes subgroup hierarchy validation and parent/visibility mutations
// across concurrent goroutines and database transactions to prevent write-skew cycles and depth races.
func WithSubgroupHierarchyLock(ctx context.Context, fn func(ctx context.Context) error) error {
	if ctx.Value(subgroupHierarchyLockCtxKey{}) != nil {
		return db.WithTx(ctx, fn)
	}
	if setting.Database.Type.IsSQLite3() && !db.InTransaction(ctx) {
		subgroupHierarchyMu.Lock()
		defer subgroupHierarchyMu.Unlock()
	}

	lockedCtx := context.WithValue(ctx, subgroupHierarchyLockCtxKey{}, true)
	return db.WithTx(lockedCtx, func(txCtx context.Context) error {
		if err := acquireSubgroupHierarchyTxLock(txCtx); err != nil {
			return fmt.Errorf("WithSubgroupHierarchyLock: %w", err)
		}
		return fn(txCtx)
	})
}

// SortOrgsLeafFirst orders organizations from deepest descendant leaf to top-level root
// so forced account purges delete child subgroups before their parent organizations.
func SortOrgsLeafFirst(ctx context.Context, orgs []*Organization) error {
	if len(orgs) <= 1 {
		return nil
	}
	depths := make(map[int64]int, len(orgs))
	for _, org := range orgs {
		if org.ParentOrgID <= 0 {
			depths[org.ID] = 0
			continue
		}
		ancestors, err := GetAncestorOrgs(ctx, org.ID)
		if err != nil {
			return err
		}
		depths[org.ID] = len(ancestors)
	}
	slices.SortFunc(orgs, func(a, b *Organization) int {
		if d := cmp.Compare(depths[b.ID], depths[a.ID]); d != 0 {
			return d
		}
		if n := cmp.Compare(a.LowerName, b.LowerName); n != 0 {
			return n
		}
		return cmp.Compare(b.ID, a.ID)
	})
	return nil
}

// MaxSubgroupDepth is the maximum allowed nesting depth below a root organization
// (root = depth 0, direct child = depth 1, ..., max leaf = depth 5).
const MaxSubgroupDepth = 5

// ErrOrgHasSubgroups indicates an organization still owns child subgroups and cannot be deleted.
type ErrOrgHasSubgroups struct {
	OrgID int64
}

func IsErrOrgHasSubgroups(err error) bool {
	_, ok := err.(ErrOrgHasSubgroups)
	return ok
}

func (err ErrOrgHasSubgroups) Error() string {
	return fmt.Sprintf("organization still has child subgroups [org_id: %d]", err.OrgID)
}

func (err ErrOrgHasSubgroups) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrSubgroupCycle indicates a parent assignment would introduce a cycle in the organization tree.
type ErrSubgroupCycle struct {
	OrgID       int64
	ParentOrgID int64
}

func IsErrSubgroupCycle(err error) bool {
	_, ok := err.(ErrSubgroupCycle)
	return ok
}

func (err ErrSubgroupCycle) Error() string {
	return fmt.Sprintf("subgroup parent assignment creates a cycle [org_id: %d, parent_org_id: %d]", err.OrgID, err.ParentOrgID)
}

func (err ErrSubgroupCycle) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrSubgroupMaxDepthExceeded indicates the subgroup hierarchy would exceed MaxSubgroupDepth.
type ErrSubgroupMaxDepthExceeded struct {
	MaxDepth int
}

func IsErrSubgroupMaxDepthExceeded(err error) bool {
	_, ok := err.(ErrSubgroupMaxDepthExceeded)
	return ok
}

func (err ErrSubgroupMaxDepthExceeded) Error() string {
	return fmt.Sprintf("subgroup nesting exceeds maximum depth of %d", err.MaxDepth)
}

func (err ErrSubgroupMaxDepthExceeded) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrSubgroupVisibilityExceedsParent indicates a subgroup's visibility is more permissive than its parent's.
type ErrSubgroupVisibilityExceedsParent struct {
	ParentVisibility structs.VisibleType
	ChildVisibility  structs.VisibleType
}

func IsErrSubgroupVisibilityExceedsParent(err error) bool {
	_, ok := err.(ErrSubgroupVisibilityExceedsParent)
	return ok
}

func (err ErrSubgroupVisibilityExceedsParent) Error() string {
	return fmt.Sprintf("subgroup visibility %q cannot be more permissive than parent organization visibility %q",
		err.ChildVisibility.String(), err.ParentVisibility.String())
}

func (err ErrSubgroupVisibilityExceedsParent) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrInvalidParentOrg indicates the specified parent ID does not refer to an existing organization.
type ErrInvalidParentOrg struct {
	ParentID int64
}

func IsErrInvalidParentOrg(err error) bool {
	_, ok := err.(ErrInvalidParentOrg)
	return ok
}

func (err ErrInvalidParentOrg) Error() string {
	return fmt.Sprintf("parent organization is invalid or does not exist [parent_id: %d]", err.ParentID)
}

func (err ErrInvalidParentOrg) Unwrap() error {
	return util.ErrInvalidArgument
}

// IsSubgroup returns true if the organization has a parent organization.
func (org *Organization) IsSubgroup() bool {
	return org != nil && org.ParentOrgID > 0
}

// GetAncestorOrgs returns the ordered chain of ancestor organizations from root to immediate parent.
func GetAncestorOrgs(ctx context.Context, orgID int64) ([]*Organization, error) {
	if orgID <= 0 {
		return nil, nil
	}
	cur, err := GetOrgByID(ctx, orgID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			return nil, ErrOrgNotExist{ID: orgID}
		}
		return nil, err
	}
	if cur.Type != user_model.UserTypeOrganization || cur.ParentOrgID <= 0 {
		return nil, nil
	}

	visited := map[int64]struct{}{cur.ID: {}}
	ancestors := make([]*Organization, 0, MaxSubgroupDepth)
	parentID := cur.ParentOrgID

	for parentID > 0 {
		if _, seen := visited[parentID]; seen {
			return nil, ErrSubgroupCycle{OrgID: orgID, ParentOrgID: parentID}
		}
		visited[parentID] = struct{}{}

		parent, err := GetOrgByID(ctx, parentID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				return nil, ErrInvalidParentOrg{ParentID: parentID}
			}
			return nil, err
		}
		if parent.Type != user_model.UserTypeOrganization {
			return nil, ErrInvalidParentOrg{ParentID: parentID}
		}
		ancestors = append(ancestors, parent)
		parentID = parent.ParentOrgID
	}

	slices.Reverse(ancestors)
	return ancestors, nil
}

// FullOrgPath returns the slash-delimited logical hierarchy path (e.g. "acme/platform/payments").
func (org *Organization) FullOrgPath(ctx context.Context) (string, error) {
	if org == nil {
		return "", nil
	}
	if org.ParentOrgID <= 0 {
		return org.Name, nil
	}
	ancestors, err := GetAncestorOrgs(ctx, org.ID)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(ancestors)+1)
	for _, anc := range ancestors {
		parts = append(parts, anc.Name)
	}
	parts = append(parts, org.Name)
	return strings.Join(parts, "/"), nil
}

// CountSubgroups returns the number of direct child subgroups of parentOrgID.
func CountSubgroups(ctx context.Context, parentOrgID int64) (int64, error) {
	if parentOrgID <= 0 {
		return 0, nil
	}
	return db.GetEngine(ctx).
		Where(builder.Eq{
			"`type`":        user_model.UserTypeOrganization,
			"parent_org_id": parentOrgID,
		}).
		Count(new(Organization))
}

// GetMaxSubtreeDepth returns the height of the subgroup subtree rooted at orgID
// (0 when orgID has no child subgroups, 1 when it has leaf children, etc.).
func GetMaxSubtreeDepth(ctx context.Context, orgID int64) (int, error) {
	if orgID <= 0 {
		return 0, nil
	}
	visited := map[int64]struct{}{orgID: {}}
	frontier := []int64{orgID}
	depth := 0

	for len(frontier) > 0 {
		var children []*Organization
		if err := db.GetEngine(ctx).
			Where(builder.Eq{"`type`": user_model.UserTypeOrganization}).
			And(builder.In("parent_org_id", frontier)).
			Find(&children); err != nil {
			return 0, err
		}
		if len(children) == 0 {
			break
		}
		depth++
		nextFrontier := make([]int64, 0, len(children))
		for _, ch := range children {
			if _, seen := visited[ch.ID]; seen {
				return 0, ErrSubgroupCycle{OrgID: ch.ID, ParentOrgID: ch.ParentOrgID}
			}
			visited[ch.ID] = struct{}{}
			nextFrontier = append(nextFrontier, ch.ID)
		}
		frontier = nextFrontier
	}
	return depth, nil
}

// ValidateParentOrg verifies that assigning childOrgID (0 if creating a new subgroup) under parentOrgID
// with childVisibility satisfies parent existence, organization type, cycle-freedom, MaxSubgroupDepth,
// and visibility monotonicity (a child cannot be more public than its parent).
func ValidateParentOrg(ctx context.Context, childOrgID, parentOrgID int64, childVisibility structs.VisibleType) (*Organization, error) {
	if parentOrgID <= 0 {
		return nil, ErrInvalidParentOrg{ParentID: parentOrgID}
	}
	if childOrgID > 0 && childOrgID == parentOrgID {
		return nil, ErrSubgroupCycle{OrgID: childOrgID, ParentOrgID: parentOrgID}
	}

	parent, err := GetOrgByID(ctx, parentOrgID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			return nil, ErrInvalidParentOrg{ParentID: parentOrgID}
		}
		return nil, err
	}
	if parent.Type != user_model.UserTypeOrganization {
		return nil, ErrInvalidParentOrg{ParentID: parentOrgID}
	}

	// VisibleTypePublic (0) < VisibleTypeLimited (1) < VisibleTypePrivate (2):
	// smaller numeric values are MORE permissive, so childVisibility < parent.Visibility is invalid.
	if childVisibility < parent.Visibility {
		return nil, ErrSubgroupVisibilityExceedsParent{
			ParentVisibility: parent.Visibility,
			ChildVisibility:  childVisibility,
		}
	}

	ancestors, err := GetAncestorOrgs(ctx, parent.ID)
	if err != nil {
		return nil, err
	}
	if childOrgID > 0 {
		for _, anc := range ancestors {
			if anc.ID == childOrgID {
				return nil, ErrSubgroupCycle{OrgID: childOrgID, ParentOrgID: parentOrgID}
			}
		}
	}

	subtreeDepth := 0
	if childOrgID > 0 {
		subtreeDepth, err = GetMaxSubtreeDepth(ctx, childOrgID)
		if err != nil {
			return nil, err
		}
	}

	// Root is depth 0; parent is at depth len(ancestors); child is at len(ancestors)+1.
	newMaxLeafDepth := len(ancestors) + 1 + subtreeDepth
	if newMaxLeafDepth > MaxSubgroupDepth {
		return nil, ErrSubgroupMaxDepthExceeded{MaxDepth: MaxSubgroupDepth}
	}

	return parent, nil
}

// GetSubgroups returns direct child subgroups of parentOrgID visible to doer, ordered alphabetically.
func GetSubgroups(ctx context.Context, parentOrgID int64, doer *user_model.User, listOpts db.ListOptions) ([]*Organization, int64, error) {
	if parentOrgID <= 0 {
		return nil, 0, nil
	}
	var allChildren []*Organization
	if err := db.GetEngine(ctx).
		Where(builder.Eq{
			"`type`":        user_model.UserTypeOrganization,
			"parent_org_id": parentOrgID,
		}).
		OrderBy("`user`.lower_name ASC").
		Find(&allChildren); err != nil {
		return nil, 0, err
	}

	visible := make([]*Organization, 0, len(allChildren))
	for _, child := range allChildren {
		if HasOrgOrUserVisible(ctx, child.AsUser(), doer) {
			visible = append(visible, child)
		}
	}

	total := int64(len(visible))
	if listOpts.PageSize > 0 {
		page := max(listOpts.Page, 1)
		start := (page - 1) * listOpts.PageSize
		if start >= len(visible) {
			return []*Organization{}, total, nil
		}
		end := min(start+listOpts.PageSize, len(visible))
		return visible[start:end], total, nil
	}
	return visible, total, nil
}

// GetDescendantOrgs returns all nested descendant subgroups under rootOrgID visible to doer
// in breadth-first alphabetical order. If includeAllForSystem is true, visibility filtering is skipped.
func GetDescendantOrgs(ctx context.Context, rootOrgID int64, doer *user_model.User, includeAllForSystem ...bool) ([]*Organization, error) {
	if rootOrgID <= 0 {
		return nil, nil
	}
	skipVis := len(includeAllForSystem) > 0 && includeAllForSystem[0]
	visited := map[int64]struct{}{rootOrgID: {}}
	frontier := []int64{rootOrgID}
	var descendants []*Organization

	for len(frontier) > 0 {
		var children []*Organization
		if err := db.GetEngine(ctx).
			Where(builder.Eq{"`type`": user_model.UserTypeOrganization}).
			And(builder.In("parent_org_id", frontier)).
			OrderBy("`user`.lower_name ASC").
			Find(&children); err != nil {
			return nil, err
		}
		if len(children) == 0 {
			break
		}
		nextFrontier := make([]int64, 0, len(children))
		for _, ch := range children {
			if _, seen := visited[ch.ID]; seen {
				return nil, ErrSubgroupCycle{OrgID: ch.ID, ParentOrgID: ch.ParentOrgID}
			}
			visited[ch.ID] = struct{}{}
			if skipVis || HasOrgOrUserVisible(ctx, ch.AsUser(), doer) {
				descendants = append(descendants, ch)
				// Only traverse below visible nodes (unless system-internal cascade).
				nextFrontier = append(nextFrontier, ch.ID)
			}
		}
		frontier = nextFrontier
	}
	return descendants, nil
}
