// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"
	gouuid "github.com/satori/go.uuid"
	ini "gopkg.in/ini.v1"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const minDBVersion = 4

// Migration describes on migration from lower version to high version
type Migration interface {
	Description() string
	Migrate(*xorm.Engine) error
}

type migration struct {
	description string
	migrate     func(*xorm.Engine) error
}

// NewMigration creates a new migration
func NewMigration(desc string, fn func(*xorm.Engine) error) Migration {
	return &migration{desc, fn}
}

// Description returns the migration's description
func (m *migration) Description() string {
	return m.description
}

// Migrate executes the migration
func (m *migration) Migrate(x *xorm.Engine) error {
	return m.migrate(x)
}

// Version describes the version table. Should have only one row with id==1
type Version struct {
	ID      int64 `xorm:"pk autoincr"`
	Version int64
}

func emptyMigration(x *xorm.Engine) error {
	return nil
}

// This is a sequence of migrations. Add new migrations to the bottom of the list.
// If you want to "retire" a migration, remove it from the top of the list and
// update minDBVersion accordingly
var migrations = []Migration{
	// v0 -> v4: before 0.6.0 -> 0.7.33
	NewMigration("fix locale file load panic", fixLocaleFileLoadPanic),                           // V4 -> V5:v0.6.0
	NewMigration("trim action compare URL prefix", trimCommitActionAppURLPrefix),                 // V5 -> V6:v0.6.3
	NewMigration("generate issue-label from issue", issueToIssueLabel),                           // V6 -> V7:v0.6.4
	NewMigration("refactor attachment table", attachmentRefactor),                                // V7 -> V8:v0.6.4
	NewMigration("rename pull request fields", renamePullRequestFields),                          // V8 -> V9:v0.6.16
	NewMigration("clean up migrate repo info", cleanUpMigrateRepoInfo),                           // V9 -> V10:v0.6.20
	NewMigration("generate rands and salt for organizations", generateOrgRandsAndSalt),           // V10 -> V11:v0.8.5
	NewMigration("convert date to unix timestamp", convertDateToUnix),                            // V11 -> V12:v0.9.2
	NewMigration("convert LDAP UseSSL option to SecurityProtocol", ldapUseSSLToSecurityProtocol), // V12 -> V13:v0.9.37

	// v13 -> v14:v0.9.87
	NewMigration("set comment updated with created", setCommentUpdatedWithCreated),
	// v14 -> v15
	NewMigration("create user column diff view style", createUserColumnDiffViewStyle),
	// v15 -> v16
	NewMigration("create user column allow create organization", createAllowCreateOrganizationColumn),
	// V16 -> v17
	NewMigration("create repo unit table and add units for all repos", addUnitsToTables),
	// v17 -> v18
	NewMigration("set protect branches updated with created", setProtectedBranchUpdatedWithCreated),
	// v18 -> v19
	NewMigration("add external login user", addExternalLoginUser),
	// v19 -> v20
	NewMigration("generate and migrate Git hooks", generateAndMigrateGitHooks),
	// v20 -> v21
	NewMigration("use new avatar path name for security reason", useNewNameAvatars),
	// v21 -> v22
	NewMigration("rewrite authorized_keys file via new format", useNewPublickeyFormat),
	// v22 -> v23
	NewMigration("generate and migrate wiki Git hooks", generateAndMigrateWikiGitHooks),
	// v23 -> v24
	NewMigration("add user openid table", addUserOpenID),
	// v24 -> v25
	NewMigration("change the key_id and primary_key_id type", changeGPGKeysColumns),
	// v25 -> v26
	NewMigration("add show field in user openid table", addUserOpenIDShow),
	// v26 -> v27
	NewMigration("generate and migrate repo and wiki Git hooks", generateAndMigrateGitHookChains),
	// v27 -> v28
	NewMigration("change mirror interval from hours to time.Duration", convertIntervalToDuration),
	// v28 -> v29
	NewMigration("add field for repo size", addRepoSize),
	// v29 -> v30
	NewMigration("add commit status table", addCommitStatus),
	// v30 -> 31
	NewMigration("add primary key to external login user", addExternalLoginUserPK),
	// v31 -> 32
	NewMigration("add field for login source synchronization", addLoginSourceSyncEnabledColumn),
	// v32 -> v33
	NewMigration("add units for team", addUnitsToRepoTeam),
	// v33 -> v34
	NewMigration("remove columns from action", removeActionColumns),
	// v34 -> v35
	NewMigration("give all units to owner teams", giveAllUnitsToOwnerTeams),
	// v35 -> v36
	NewMigration("adds comment to an action", addCommentIDToAction),
	// v36 -> v37
	NewMigration("regenerate git hooks", regenerateGitHooks36),
	// v37 -> v38
	NewMigration("unescape user full names", unescapeUserFullNames),
	// v38 -> v39
	NewMigration("remove commits and settings unit types", removeCommitsUnitType),
	// v39 -> v40
	NewMigration("add tags to releases and sync existing repositories", releaseAddColumnIsTagAndSyncTags),
	// v40 -> v41
	NewMigration("fix protected branch can push value to false", fixProtectedBranchCanPushValue),
	// v41 -> v42
	NewMigration("remove duplicate unit types", removeDuplicateUnitTypes),
	// v42 -> v43
	NewMigration("empty step", emptyMigration),
	// v43 -> v44
	NewMigration("empty step", emptyMigration),
	// v44 -> v45
	NewMigration("empty step", emptyMigration),
	// v45 -> v46
	NewMigration("remove index column from repo_unit table", removeIndexColumnFromRepoUnitTable),
	// v46 -> v47
	NewMigration("remove organization watch repositories", removeOrganizationWatchRepo),
	// v47 -> v48
	NewMigration("add deleted branches", addDeletedBranch),
	// v48 -> v49
	NewMigration("add repo indexer status", addRepoIndexerStatus),
	// v49 -> v50
	NewMigration("adds time tracking and stopwatches", addTimetracking),
	// v50 -> v51
	NewMigration("migrate protected branch struct", migrateProtectedBranchStruct),
	// v51 -> v52
	NewMigration("add default value to user prohibit_login", addDefaultValueToUserProhibitLogin),
	// v52 -> v53
	NewMigration("add lfs lock table", addLFSLock),
	// v53 -> v54
	NewMigration("add reactions", addReactions),
	// v54 -> v55
	NewMigration("add pull request options", addPullRequestOptions),
	// v55 -> v56
	NewMigration("add writable deploy keys", addModeToDeploKeys),
	// v56 -> v57
	NewMigration("remove is_owner, num_teams columns from org_user", removeIsOwnerColumnFromOrgUser),
	// v57 -> v58
	NewMigration("add closed_unix column for issues", addIssueClosedTime),
	// v58 -> v59
	NewMigration("add label descriptions", addLabelsDescriptions),
	// v59 -> v60
	NewMigration("add merge whitelist for protected branches", addProtectedBranchMergeWhitelist),
	// v60 -> v61
	NewMigration("add is_fsck_enabled column for repos", addFsckEnabledToRepo),
	// v61 -> v62
	NewMigration("add size column for attachments", addSizeToAttachment),
	// v62 -> v63
	NewMigration("add last used passcode column for TOTP", addLastUsedPasscodeTOTP),
	// v63 -> v64
	NewMigration("add language column for user setting", addLanguageSetting),
	// v64 -> v65
	NewMigration("add multiple assignees", addMultipleAssignees),
	// v65 -> v66
	NewMigration("add u2f", addU2FReg),
	// v66 -> v67
	NewMigration("add login source id column for public_key table", addLoginSourceIDToPublicKeyTable),
	// v67 -> v68
	NewMigration("remove stale watches", removeStaleWatches),
	// v68 -> V69
	NewMigration("Reformat and remove incorrect topics", reformatAndRemoveIncorrectTopics),
	// v69 -> v70
	NewMigration("move team units to team_unit table", moveTeamUnitsToTeamUnitTable),
	// v70 -> v71
	NewMigration("add issue_dependencies", addIssueDependencies),
	// v71 -> v72
	NewMigration("protect each scratch token", addScratchHash),
	// v72 -> v73
	NewMigration("add review", addReview),
	// v73 -> v74
	NewMigration("add must_change_password column for users table", addMustChangePassword),
	// v74 -> v75
	NewMigration("add approval whitelists to protected branches", addApprovalWhitelistsToProtectedBranches),
	// v75 -> v76
	NewMigration("clear nonused data which not deleted when user was deleted", clearNonusedData),
	// v76 -> v77
	NewMigration("add pull request rebase with merge commit", addPullRequestRebaseWithMerge),
	// v77 -> v78
	NewMigration("add theme to users", addUserDefaultTheme),
	// v78 -> v79
	NewMigration("rename repo is_bare to repo is_empty", renameRepoIsBareToIsEmpty),
	// v79 -> v80
	NewMigration("add can close issues via commit in any branch", addCanCloseIssuesViaCommitInAnyBranch),
	// v80 -> v81
	NewMigration("add is locked to issues", addIsLockedToIssues),
	// v81 -> v82
	NewMigration("update U2F counter type", changeU2FCounterType),
	// v82 -> v83
	NewMigration("hot fix for wrong release sha1 on release table", fixReleaseSha1OnReleaseTable),
}

// Migrate database to current version
func Migrate(x *xorm.Engine) error {
	if err := x.Sync(new(Version)); err != nil {
		return fmt.Errorf("sync: %v", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return fmt.Errorf("get: %v", err)
	} else if !has {
		// If the version record does not exist we think
		// it is a fresh installation and we can skip all migrations.
		currentVersion.ID = 0
		currentVersion.Version = int64(minDBVersion + len(migrations))

		if _, err = x.InsertOne(currentVersion); err != nil {
			return fmt.Errorf("insert: %v", err)
		}
	}

	v := currentVersion.Version
	if minDBVersion > v {
		log.Fatal(4, `Gitea no longer supports auto-migration from your previously installed version.
Please try to upgrade to a lower version (>= v0.6.0) first, then upgrade to current version.`)
		return nil
	}

	if int(v-minDBVersion) > len(migrations) {
		// User downgraded Gitea.
		currentVersion.Version = int64(len(migrations) + minDBVersion)
		_, err = x.ID(1).Update(currentVersion)
		return err
	}
	for i, m := range migrations[v-minDBVersion:] {
		log.Info("Migration: %s", m.Description())
		if err = m.Migrate(x); err != nil {
			return fmt.Errorf("do migrate: %v", err)
		}
		currentVersion.Version = v + int64(i) + 1
		if _, err = x.ID(1).Update(currentVersion); err != nil {
			return err
		}
	}
	return nil
}

func dropTableColumns(sess *xorm.Session, tableName string, columnNames ...string) (err error) {
	if tableName == "" || len(columnNames) == 0 {
		return nil
	}

	switch {
	case setting.UseSQLite3:
		log.Warn("Unable to drop columns in SQLite")
	case setting.UseMySQL, setting.UseTiDB, setting.UsePostgreSQL:
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "`"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.UseMSSQL:
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "`" + strings.ToLower(col) + "`"
		}
		sql := fmt.Sprintf("SELECT Name FROM SYS.DEFAULT_CONSTRAINTS WHERE PARENT_OBJECT_ID = OBJECT_ID('%[1]s') AND PARENT_COLUMN_ID IN (SELECT column_id FROM sys.columns WHERE lower(NAME) IN (%[2]s) AND object_id = OBJECT_ID('%[1]s'))",
			tableName, strings.Replace(cols, "`", "'", -1))
		constraints := make([]string, 0)
		if err := sess.SQL(sql).Find(&constraints); err != nil {
			sess.Rollback()
			return fmt.Errorf("Find constraints: %v", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT `%s`", tableName, constraint)); err != nil {
				sess.Rollback()
				return fmt.Errorf("Drop table `%s` constraint `%s`: %v", tableName, constraint, err)
			}
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s", tableName, cols)); err != nil {
			sess.Rollback()
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}

		return sess.Commit()
	default:
		log.Fatal(4, "Unrecognized DB")
	}

	return nil
}

func fixLocaleFileLoadPanic(_ *xorm.Engine) error {
	cfg, err := ini.Load(setting.CustomConf)
	if err != nil {
		return fmt.Errorf("load custom config: %v", err)
	}

	cfg.DeleteSection("i18n")
	if err = cfg.SaveTo(setting.CustomConf); err != nil {
		return fmt.Errorf("save custom config: %v", err)
	}

	setting.Langs = strings.Split(strings.Replace(strings.Join(setting.Langs, ","), "fr-CA", "fr-FR", 1), ",")
	return nil
}

func trimCommitActionAppURLPrefix(x *xorm.Engine) error {
	type PushCommit struct {
		Sha1        string
		Message     string
		AuthorEmail string
		AuthorName  string
	}

	type PushCommits struct {
		Len        int
		Commits    []*PushCommit
		CompareURL string `json:"CompareUrl"`
	}

	type Action struct {
		ID      int64  `xorm:"pk autoincr"`
		Content string `xorm:"TEXT"`
	}

	results, err := x.Query("SELECT `id`,`content` FROM `action` WHERE `op_type`=?", 5)
	if err != nil {
		return fmt.Errorf("select commit actions: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	var pushCommits *PushCommits
	for _, action := range results {
		actID := com.StrTo(string(action["id"])).MustInt64()
		if actID == 0 {
			continue
		}

		pushCommits = new(PushCommits)
		if err = json.Unmarshal(action["content"], pushCommits); err != nil {
			return fmt.Errorf("unmarshal action content[%d]: %v", actID, err)
		}

		infos := strings.Split(pushCommits.CompareURL, "/")
		if len(infos) <= 4 {
			continue
		}
		pushCommits.CompareURL = strings.Join(infos[len(infos)-4:], "/")

		p, err := json.Marshal(pushCommits)
		if err != nil {
			return fmt.Errorf("marshal action content[%d]: %v", actID, err)
		}

		if _, err = sess.Id(actID).Update(&Action{
			Content: string(p),
		}); err != nil {
			return fmt.Errorf("update action[%d]: %v", actID, err)
		}
	}
	return sess.Commit()
}

func issueToIssueLabel(x *xorm.Engine) error {
	type IssueLabel struct {
		ID      int64 `xorm:"pk autoincr"`
		IssueID int64 `xorm:"UNIQUE(s)"`
		LabelID int64 `xorm:"UNIQUE(s)"`
	}

	issueLabels := make([]*IssueLabel, 0, 50)
	results, err := x.Query("SELECT `id`,`label_ids` FROM `issue`")
	if err != nil {
		if strings.Contains(err.Error(), "no such column") ||
			strings.Contains(err.Error(), "Unknown column") {
			return nil
		}
		return fmt.Errorf("select issues: %v", err)
	}
	for _, issue := range results {
		issueID := com.StrTo(issue["id"]).MustInt64()

		// Just in case legacy code can have duplicated IDs for same label.
		mark := make(map[int64]bool)
		for _, idStr := range strings.Split(string(issue["label_ids"]), "|") {
			labelID := com.StrTo(strings.TrimPrefix(idStr, "$")).MustInt64()
			if labelID == 0 || mark[labelID] {
				continue
			}

			mark[labelID] = true
			issueLabels = append(issueLabels, &IssueLabel{
				IssueID: issueID,
				LabelID: labelID,
			})
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = sess.Sync2(new(IssueLabel)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	} else if _, err = sess.Insert(issueLabels); err != nil {
		return fmt.Errorf("insert issue-labels: %v", err)
	}

	return sess.Commit()
}

func attachmentRefactor(x *xorm.Engine) error {
	type Attachment struct {
		ID   int64  `xorm:"pk autoincr"`
		UUID string `xorm:"uuid INDEX"`

		// For rename purpose.
		Path    string `xorm:"-"`
		NewPath string `xorm:"-"`
	}

	results, err := x.Query("SELECT * FROM `attachment`")
	if err != nil {
		return fmt.Errorf("select attachments: %v", err)
	}

	attachments := make([]*Attachment, 0, len(results))
	for _, attach := range results {
		if !com.IsExist(string(attach["path"])) {
			// If the attachment is already missing, there is no point to update it.
			continue
		}
		attachments = append(attachments, &Attachment{
			ID:   com.StrTo(attach["id"]).MustInt64(),
			UUID: gouuid.NewV4().String(),
			Path: string(attach["path"]),
		})
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = sess.Sync2(new(Attachment)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	// Note: Roll back for rename can be a dead loop,
	// 	so produces a backup file.
	var buf bytes.Buffer
	buf.WriteString("# old path -> new path\n")

	// Update database first because this is where error happens the most often.
	for _, attach := range attachments {
		if _, err = sess.Id(attach.ID).Update(attach); err != nil {
			return err
		}

		attach.NewPath = path.Join(setting.AttachmentPath, attach.UUID[0:1], attach.UUID[1:2], attach.UUID)
		buf.WriteString(attach.Path)
		buf.WriteString("\t")
		buf.WriteString(attach.NewPath)
		buf.WriteString("\n")
	}

	// Then rename attachments.
	isSucceed := true
	defer func() {
		if isSucceed {
			return
		}

		dumpPath := path.Join(setting.LogRootPath, "attachment_path.dump")
		ioutil.WriteFile(dumpPath, buf.Bytes(), 0666)
		log.Info("Failed to rename some attachments, old and new paths are saved into: %s", dumpPath)
	}()
	for _, attach := range attachments {
		if err = os.MkdirAll(path.Dir(attach.NewPath), os.ModePerm); err != nil {
			isSucceed = false
			return err
		}

		if err = os.Rename(attach.Path, attach.NewPath); err != nil {
			isSucceed = false
			return err
		}
	}

	return sess.Commit()
}

func renamePullRequestFields(x *xorm.Engine) (err error) {
	type PullRequest struct {
		ID         int64 `xorm:"pk autoincr"`
		PullID     int64 `xorm:"INDEX"`
		PullIndex  int64
		HeadBarcnh string

		IssueID    int64 `xorm:"INDEX"`
		Index      int64
		HeadBranch string
	}

	if err = x.Sync(new(PullRequest)); err != nil {
		return fmt.Errorf("sync: %v", err)
	}

	results, err := x.Query("SELECT `id`,`pull_id`,`pull_index`,`head_barcnh` FROM `pull_request`")
	if err != nil {
		if strings.Contains(err.Error(), "no such column") {
			return nil
		}
		return fmt.Errorf("select pull requests: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	var pull *PullRequest
	for _, pr := range results {
		pull = &PullRequest{
			ID:         com.StrTo(pr["id"]).MustInt64(),
			IssueID:    com.StrTo(pr["pull_id"]).MustInt64(),
			Index:      com.StrTo(pr["pull_index"]).MustInt64(),
			HeadBranch: string(pr["head_barcnh"]),
		}
		if pull.Index == 0 {
			continue
		}
		if _, err = sess.Id(pull.ID).Update(pull); err != nil {
			return err
		}
	}

	return sess.Commit()
}

func cleanUpMigrateRepoInfo(x *xorm.Engine) (err error) {
	type (
		User struct {
			ID        int64 `xorm:"pk autoincr"`
			LowerName string
		}
		Repository struct {
			ID        int64 `xorm:"pk autoincr"`
			OwnerID   int64
			LowerName string
		}
	)

	repos := make([]*Repository, 0, 25)
	if err = x.Where("is_mirror=?", false).Find(&repos); err != nil {
		return fmt.Errorf("select all non-mirror repositories: %v", err)
	}
	var user *User
	for _, repo := range repos {
		user = &User{ID: repo.OwnerID}
		has, err := x.Get(user)
		if err != nil {
			return fmt.Errorf("get owner of repository[%d - %d]: %v", repo.ID, repo.OwnerID, err)
		} else if !has {
			continue
		}

		configPath := filepath.Join(setting.RepoRootPath, user.LowerName, repo.LowerName+".git/config")

		// In case repository file is somehow missing.
		if !com.IsFile(configPath) {
			continue
		}

		cfg, err := ini.Load(configPath)
		if err != nil {
			return fmt.Errorf("open config file: %v", err)
		}
		cfg.DeleteSection("remote \"origin\"")
		if err = cfg.SaveToIndent(configPath, "\t"); err != nil {
			return fmt.Errorf("save config file: %v", err)
		}
	}

	return nil
}

func generateOrgRandsAndSalt(x *xorm.Engine) (err error) {
	type User struct {
		ID    int64  `xorm:"pk autoincr"`
		Rands string `xorm:"VARCHAR(10)"`
		Salt  string `xorm:"VARCHAR(10)"`
	}

	orgs := make([]*User, 0, 10)
	if err = x.Where("type=1").And("rands=''").Find(&orgs); err != nil {
		return fmt.Errorf("select all organizations: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	for _, org := range orgs {
		if org.Rands, err = generate.GetRandomString(10); err != nil {
			return err
		}
		if org.Salt, err = generate.GetRandomString(10); err != nil {
			return err
		}
		if _, err = sess.Id(org.ID).Update(org); err != nil {
			return err
		}
	}

	return sess.Commit()
}

// TAction defines the struct for migrating table action
type TAction struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TAction) TableName() string { return "action" }

// TNotice defines the struct for migrating table notice
type TNotice struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TNotice) TableName() string { return "notice" }

// TComment defines the struct for migrating table comment
type TComment struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TComment) TableName() string { return "comment" }

// TIssue defines the struct for migrating table issue
type TIssue struct {
	ID           int64 `xorm:"pk autoincr"`
	DeadlineUnix int64
	CreatedUnix  int64
	UpdatedUnix  int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TIssue) TableName() string { return "issue" }

// TMilestone defines the struct for migrating table milestone
type TMilestone struct {
	ID             int64 `xorm:"pk autoincr"`
	DeadlineUnix   int64
	ClosedDateUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TMilestone) TableName() string { return "milestone" }

// TAttachment defines the struct for migrating table attachment
type TAttachment struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TAttachment) TableName() string { return "attachment" }

// TLoginSource defines the struct for migrating table login_source
type TLoginSource struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
	UpdatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TLoginSource) TableName() string { return "login_source" }

// TPull defines the struct for migrating table pull_request
type TPull struct {
	ID         int64 `xorm:"pk autoincr"`
	MergedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TPull) TableName() string { return "pull_request" }

// TRelease defines the struct for migrating table release
type TRelease struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TRelease) TableName() string { return "release" }

// TRepo defines the struct for migrating table repository
type TRepo struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
	UpdatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TRepo) TableName() string { return "repository" }

// TMirror defines the struct for migrating table mirror
type TMirror struct {
	ID             int64 `xorm:"pk autoincr"`
	UpdatedUnix    int64
	NextUpdateUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TMirror) TableName() string { return "mirror" }

// TPublicKey defines the struct for migrating table public_key
type TPublicKey struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
	UpdatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TPublicKey) TableName() string { return "public_key" }

// TDeployKey defines the struct for migrating table deploy_key
type TDeployKey struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
	UpdatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TDeployKey) TableName() string { return "deploy_key" }

// TAccessToken defines the struct for migrating table access_token
type TAccessToken struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
	UpdatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TAccessToken) TableName() string { return "access_token" }

// TUser defines the struct for migrating table user
type TUser struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
	UpdatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TUser) TableName() string { return "user" }

// TWebhook defines the struct for migrating table webhook
type TWebhook struct {
	ID          int64 `xorm:"pk autoincr"`
	CreatedUnix int64
	UpdatedUnix int64
}

// TableName will be invoked by XORM to customrize the table name
func (t *TWebhook) TableName() string { return "webhook" }

func convertDateToUnix(x *xorm.Engine) (err error) {
	log.Info("This migration could take up to minutes, please be patient.")
	type Bean struct {
		ID         int64 `xorm:"pk autoincr"`
		Created    time.Time
		Updated    time.Time
		Merged     time.Time
		Deadline   time.Time
		ClosedDate time.Time
		NextUpdate time.Time
	}

	var tables = []struct {
		name string
		cols []string
		bean interface{}
	}{
		{"action", []string{"created"}, new(TAction)},
		{"notice", []string{"created"}, new(TNotice)},
		{"comment", []string{"created"}, new(TComment)},
		{"issue", []string{"deadline", "created", "updated"}, new(TIssue)},
		{"milestone", []string{"deadline", "closed_date"}, new(TMilestone)},
		{"attachment", []string{"created"}, new(TAttachment)},
		{"login_source", []string{"created", "updated"}, new(TLoginSource)},
		{"pull_request", []string{"merged"}, new(TPull)},
		{"release", []string{"created"}, new(TRelease)},
		{"repository", []string{"created", "updated"}, new(TRepo)},
		{"mirror", []string{"updated", "next_update"}, new(TMirror)},
		{"public_key", []string{"created", "updated"}, new(TPublicKey)},
		{"deploy_key", []string{"created", "updated"}, new(TDeployKey)},
		{"access_token", []string{"created", "updated"}, new(TAccessToken)},
		{"user", []string{"created", "updated"}, new(TUser)},
		{"webhook", []string{"created", "updated"}, new(TWebhook)},
	}

	for _, table := range tables {
		log.Info("Converting table: %s", table.name)
		if err = x.Sync2(table.bean); err != nil {
			return fmt.Errorf("Sync [table: %s]: %v", table.name, err)
		}

		offset := 0
		for {
			beans := make([]*Bean, 0, 100)
			if err = x.Table(table.name).Asc("id").Limit(100, offset).Find(&beans); err != nil {
				return fmt.Errorf("select beans [table: %s, offset: %d]: %v", table.name, offset, err)
			}
			log.Trace("Table [%s]: offset: %d, beans: %d", table.name, offset, len(beans))
			if len(beans) == 0 {
				break
			}
			offset += 100

			baseSQL := "UPDATE `" + table.name + "` SET "
			for _, bean := range beans {
				valSQLs := make([]string, 0, len(table.cols))
				for _, col := range table.cols {
					fieldSQL := ""
					fieldSQL += col + "_unix = "

					switch col {
					case "deadline":
						if bean.Deadline.IsZero() {
							continue
						}
						fieldSQL += com.ToStr(bean.Deadline.Unix())
					case "created":
						fieldSQL += com.ToStr(bean.Created.Unix())
					case "updated":
						fieldSQL += com.ToStr(bean.Updated.Unix())
					case "closed_date":
						fieldSQL += com.ToStr(bean.ClosedDate.Unix())
					case "merged":
						fieldSQL += com.ToStr(bean.Merged.Unix())
					case "next_update":
						fieldSQL += com.ToStr(bean.NextUpdate.Unix())
					}

					valSQLs = append(valSQLs, fieldSQL)
				}

				if len(valSQLs) == 0 {
					continue
				}

				if _, err = x.Exec(baseSQL + strings.Join(valSQLs, ",") + " WHERE id = " + com.ToStr(bean.ID)); err != nil {
					return fmt.Errorf("update bean [table: %s, id: %d]: %v", table.name, bean.ID, err)
				}
			}
		}
	}

	return nil
}
