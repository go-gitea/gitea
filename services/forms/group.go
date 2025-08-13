package forms

import "code.gitea.io/gitea/modules/structs"

// CreateGroupForm form for creating a repository group
type CreateGroupForm struct {
	GroupName          string `binding:"Required;MaxSize(255)"`
	Description        string `binding:"MaxSize(2048)"`
	Permission         string
	CanCreateGroupRepo bool
	ParentGroupID      int64
}

type MovedGroupItemForm struct {
	IsGroup   bool  `json:"isGroup"`
	ItemID    int64 `json:"id"`
	NewParent int64 `json:"newParent"`
	NewPos    int   `json:"newPosition"`
}
type CreateGroupTeamForm struct {
	Permission              string
	Access                  string
	CanCreateRepoOrSubGroup bool
}

type UpdateGroupSettingForm struct {
	Name        string `binding:"Required;MaxSize(255)" locale:"group.group_name_holder"`
	Description string `binding:"MaxSize(2048)"`
	Visibility  structs.VisibleType
}
