// Package organization implements nested subgroups and hierarchical permission inheritance for Gitea.
// Bounty: go-gitea/gitea #1872 ($500 USD — Subgroups in Gitea)
package organization

import (
	"errors"
	"strings"
)

var (
	ErrSubgroupCycleDetected = errors.New("organization subgroup hierarchy cycle detected")
	ErrMaxSubgroupDepth      = errors.New("maximum subgroup nesting depth exceeded")
	ErrInvalidSubgroupName   = errors.New("invalid subgroup path segment")
)

const MaxSubgroupDepth = 5

type AccessMode int

const (
	AccessModeNone AccessMode = iota
	AccessModeRead
	AccessModeWrite
	AccessModeAdmin
	AccessModeOwner
)

// OrgGroupNode represents either a top-level Organization (ParentID == 0) or a nested Subgroup.
type OrgGroupNode struct {
	ID          int64
	ParentID    int64
	Name        string
	LowerName   string
	FullPath    string
	Description string
	DirectRoles map[int64]AccessMode // userID -> AccessMode
}

type SubgroupRegistry struct {
	nodes map[int64]*OrgGroupNode
}

func NewSubgroupRegistry() *SubgroupRegistry {
	return &SubgroupRegistry{
		nodes: make(map[int64]*OrgGroupNode),
	}
}

// CreateGroup registers a root organization or nested subgroup, enforcing depth and cycle invariants.
func (r *SubgroupRegistry) CreateGroup(id, parentID int64, name, description string) (*OrgGroupNode, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.Contains(trimmed, "/") {
		return nil, ErrInvalidSubgroupName
	}

	fullPath := strings.ToLower(trimmed)
	depth := 1
	currParentID := parentID

	visited := map[int64]bool{id: true}
	var pathSegments []string
	pathSegments = append(pathSegments, fullPath)

	for currParentID != 0 {
		if visited[currParentID] {
			return nil, ErrSubgroupCycleDetected
		}
		visited[currParentID] = true
		parent, ok := r.nodes[currParentID]
		if !ok {
			return nil, errors.New("parent organization group not found")
		}
		pathSegments = append([]string{parent.LowerName}, pathSegments...)
		currParentID = parent.ParentID
		depth++
		if depth > MaxSubgroupDepth {
			return nil, ErrMaxSubgroupDepth
		}
	}

	node := &OrgGroupNode{
		ID:          id,
		ParentID:    parentID,
		Name:        trimmed,
		LowerName:   strings.ToLower(trimmed),
		FullPath:    strings.Join(pathSegments, "/"),
		Description: description,
		DirectRoles: make(map[int64]AccessMode),
	}
	r.nodes[id] = node
	return node, nil
}

// ResolveEffectiveAccess computes a user's highest inherited AccessMode walking from the subgroup up to the root Org.
func (r *SubgroupRegistry) ResolveEffectiveAccess(groupID, userID int64) AccessMode {
	highest := AccessModeNone
	currID := groupID
	visited := make(map[int64]bool)

	for currID != 0 && !visited[currID] {
		visited[currID] = true
		node, ok := r.nodes[currID]
		if !ok {
			break
		}
		if mode, exists := node.DirectRoles[userID]; exists && mode > highest {
			highest = mode
		}
		currID = node.ParentID
	}
	return highest
}
