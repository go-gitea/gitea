// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"container/list"
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	// Needed for jpeg support
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Unknwon/com"
	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"
	"github.com/nfnt/resize"
	"golang.org/x/crypto/pbkdf2"

	"code.gitea.io/git"
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// UserType defines the user type
type UserType int

const (
	// UserTypeIndividual defines an individual user
	UserTypeIndividual UserType = iota // Historic reason to make it starts at 0.

	// UserTypeOrganization defines an organization
	UserTypeOrganization
)

const syncExternalUsers = "sync_external_users"

var (
	// ErrUserNotKeyOwner user does not own this key error
	ErrUserNotKeyOwner = errors.New("User does not own this public key")

	// ErrEmailNotExist e-mail does not exist error
	ErrEmailNotExist = errors.New("E-mail does not exist")

	// ErrEmailNotActivated e-mail address has not been activated error
	ErrEmailNotActivated = errors.New("E-mail address has not been activated")

	// ErrUserNameIllegal user name contains illegal characters error
	ErrUserNameIllegal = errors.New("User name contains illegal characters")

	// ErrLoginSourceNotActived login source is not actived error
	ErrLoginSourceNotActived = errors.New("Login source is not actived")

	// ErrUnsupportedLoginType login source is unknown error
	ErrUnsupportedLoginType = errors.New("Login source is unknown")
)

// User represents the object of individual and member of organization.
type User struct {
	ID        int64  `xorm:"pk autoincr"`
	LowerName string `xorm:"UNIQUE NOT NULL"`
	Name      string `xorm:"UNIQUE NOT NULL"`
	FullName  string
	// Email is the primary email address (to be used for communication)
	Email            string `xorm:"NOT NULL"`
	KeepEmailPrivate bool
	Passwd           string `xorm:"NOT NULL"`
	LoginType        LoginType
	LoginSource      int64 `xorm:"NOT NULL DEFAULT 0"`
	LoginName        string
	Type             UserType
	OwnedOrgs        []*User       `xorm:"-"`
	Orgs             []*User       `xorm:"-"`
	Repos            []*Repository `xorm:"-"`
	Location         string
	Website          string
	Rands            string `xorm:"VARCHAR(10)"`
	Salt             string `xorm:"VARCHAR(10)"`

	Created       time.Time `xorm:"-"`
	CreatedUnix   int64     `xorm:"INDEX created"`
	Updated       time.Time `xorm:"-"`
	UpdatedUnix   int64     `xorm:"INDEX updated"`
	LastLogin     time.Time `xorm:"-"`
	LastLoginUnix int64     `xorm:"INDEX"`

	// Remember visibility choice for convenience, true for private
	LastRepoVisibility bool
	// Maximum repository creation limit, -1 means use global default
	MaxRepoCreation int `xorm:"NOT NULL DEFAULT -1"`

	// Permissions
	IsActive                bool `xorm:"INDEX"` // Activate primary email
	IsAdmin                 bool
	AllowGitHook            bool
	AllowImportLocal        bool // Allow migrate repository by local path
	AllowCreateOrganization bool `xorm:"DEFAULT true"`
	ProhibitLogin           bool

	// Avatar
	Avatar          string `xorm:"VARCHAR(2048) NOT NULL"`
	AvatarEmail     string `xorm:"NOT NULL"`
	UseCustomAvatar bool

	// Counters
	NumFollowers int
	NumFollowing int `xorm:"NOT NULL DEFAULT 0"`
	NumStars     int
	NumRepos     int

	// For organization
	Description string
	NumTeams    int
	NumMembers  int
	Teams       []*Team `xorm:"-"`
	Members     []*User `xorm:"-"`

	// Preferences
	DiffViewStyle string `xorm:"NOT NULL DEFAULT ''"`
}

// BeforeUpdate is invoked from XORM before updating this object.
func (u *User) BeforeUpdate() {
	if u.MaxRepoCreation < -1 {
		u.MaxRepoCreation = -1
	}
}

// SetLastLogin set time to last login
func (u *User) SetLastLogin() {
	u.LastLoginUnix = time.Now().Unix()
}

// UpdateDiffViewStyle updates the users diff view style
func (u *User) UpdateDiffViewStyle(style string) error {
	u.DiffViewStyle = style
	return UpdateUserCols(u, "diff_view_style")
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (u *User) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		u.Created = time.Unix(u.CreatedUnix, 0).Local()
	case "updated_unix":
		u.Updated = time.Unix(u.UpdatedUnix, 0).Local()
	case "last_login_unix":
		u.LastLogin = time.Unix(u.LastLoginUnix, 0).Local()
	}
}

// getEmail returns an noreply email, if the user has set to keep his
// email address private, otherwise the primary email address.
func (u *User) getEmail() string {
	if u.KeepEmailPrivate {
		return fmt.Sprintf("%s@%s", u.LowerName, setting.Service.NoReplyAddress)
	}
	return u.Email
}

// APIFormat converts a User to api.User
func (u *User) APIFormat() *api.User {
	return &api.User{
		ID:        u.ID,
		UserName:  u.Name,
		FullName:  u.FullName,
		Email:     u.getEmail(),
		AvatarURL: u.AvatarLink(),
	}
}

// IsLocal returns true if user login type is LoginPlain.
func (u *User) IsLocal() bool {
	return u.LoginType <= LoginPlain
}

// IsOAuth2 returns true if user login type is LoginOAuth2.
func (u *User) IsOAuth2() bool {
	return u.LoginType == LoginOAuth2
}

// HasForkedRepo checks if user has already forked a repository with given ID.
func (u *User) HasForkedRepo(repoID int64) bool {
	_, has := HasForkedRepo(u.ID, repoID)
	return has
}

// MaxCreationLimit returns the number of repositories a user is allowed to create
func (u *User) MaxCreationLimit() int {
	if u.MaxRepoCreation <= -1 {
		return setting.Repository.MaxCreationLimit
	}
	return u.MaxRepoCreation
}

// CanCreateRepo returns if user login can create a repository
func (u *User) CanCreateRepo() bool {
	if u.IsAdmin {
		return true
	}
	if u.MaxRepoCreation <= -1 {
		if setting.Repository.MaxCreationLimit <= -1 {
			return true
		}
		return u.NumRepos < setting.Repository.MaxCreationLimit
	}
	return u.NumRepos < u.MaxRepoCreation
}

// CanCreateOrganization returns true if user can create organisation.
func (u *User) CanCreateOrganization() bool {
	return u.IsAdmin || (u.AllowCreateOrganization && !setting.Admin.DisableRegularOrgCreation)
}

// CanEditGitHook returns true if user can edit Git hooks.
func (u *User) CanEditGitHook() bool {
	return !setting.DisableGitHooks && (u.IsAdmin || u.AllowGitHook)
}

// CanImportLocal returns true if user can migrate repository by local path.
func (u *User) CanImportLocal() bool {
	if !setting.ImportLocalPaths {
		return false
	}
	return u.IsAdmin || u.AllowImportLocal
}

// DashboardLink returns the user dashboard page link.
func (u *User) DashboardLink() string {
	if u.IsOrganization() {
		return setting.AppSubURL + "/org/" + u.Name + "/dashboard/"
	}
	return setting.AppSubURL + "/"
}

// HomeLink returns the user or organization home page link.
func (u *User) HomeLink() string {
	return setting.AppSubURL + "/" + u.Name
}

// HTMLURL returns the user or organization's full link.
func (u *User) HTMLURL() string {
	return setting.AppURL + u.Name
}

// GenerateEmailActivateCode generates an activate code based on user information and given e-mail.
func (u *User) GenerateEmailActivateCode(email string) string {
	code := base.CreateTimeLimitCode(
		com.ToStr(u.ID)+email+u.LowerName+u.Passwd+u.Rands,
		setting.Service.ActiveCodeLives, nil)

	// Add tail hex username
	code += hex.EncodeToString([]byte(u.LowerName))
	return code
}

// GenerateActivateCode generates an activate code based on user information.
func (u *User) GenerateActivateCode() string {
	return u.GenerateEmailActivateCode(u.Email)
}

// CustomAvatarPath returns user custom avatar file path.
func (u *User) CustomAvatarPath() string {
	return filepath.Join(setting.AvatarUploadPath, u.Avatar)
}

// GenerateRandomAvatar generates a random avatar for user.
func (u *User) GenerateRandomAvatar() error {
	return u.generateRandomAvatar(x)
}

func (u *User) generateRandomAvatar(e Engine) error {
	seed := u.Email
	if len(seed) == 0 {
		seed = u.Name
	}

	img, err := avatar.RandomImage([]byte(seed))
	if err != nil {
		return fmt.Errorf("RandomImage: %v", err)
	}
	// NOTICE for random avatar, it still uses id as avatar name, but custom avatar use md5
	// since random image is not a user's photo, there is no security for enumable
	u.Avatar = fmt.Sprintf("%d", u.ID)
	if err = os.MkdirAll(filepath.Dir(u.CustomAvatarPath()), os.ModePerm); err != nil {
		return fmt.Errorf("MkdirAll: %v", err)
	}
	fw, err := os.Create(u.CustomAvatarPath())
	if err != nil {
		return fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()

	if _, err := e.Id(u.ID).Cols("avatar").Update(u); err != nil {
		return err
	}

	if err = png.Encode(fw, img); err != nil {
		return fmt.Errorf("Encode: %v", err)
	}

	log.Info("New random avatar created: %d", u.ID)
	return nil
}

// RelAvatarLink returns relative avatar link to the site domain,
// which includes app sub-url as prefix. However, it is possible
// to return full URL if user enables Gravatar-like service.
func (u *User) RelAvatarLink() string {
	if u.ID == -1 {
		return base.DefaultAvatarLink()
	}

	switch {
	case u.UseCustomAvatar:
		if !com.IsFile(u.CustomAvatarPath()) {
			return base.DefaultAvatarLink()
		}
		return setting.AppSubURL + "/avatars/" + u.Avatar
	case setting.DisableGravatar, setting.OfflineMode:
		if !com.IsFile(u.CustomAvatarPath()) {
			if err := u.GenerateRandomAvatar(); err != nil {
				log.Error(3, "GenerateRandomAvatar: %v", err)
			}
		}

		return setting.AppSubURL + "/avatars/" + u.Avatar
	}
	return base.AvatarLink(u.AvatarEmail)
}

// AvatarLink returns user avatar absolute link.
func (u *User) AvatarLink() string {
	link := u.RelAvatarLink()
	if link[0] == '/' && link[1] != '/' {
		return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL)[1:]
	}
	return link
}

// GetFollowers returns range of user's followers.
func (u *User) GetFollowers(page int) ([]*User, error) {
	users := make([]*User, 0, ItemsPerPage)
	sess := x.
		Limit(ItemsPerPage, (page-1)*ItemsPerPage).
		Where("follow.follow_id=?", u.ID)
	if setting.UsePostgreSQL {
		sess = sess.Join("LEFT", "follow", `"user".id=follow.user_id`)
	} else {
		sess = sess.Join("LEFT", "follow", "user.id=follow.user_id")
	}
	return users, sess.Find(&users)
}

// IsFollowing returns true if user is following followID.
func (u *User) IsFollowing(followID int64) bool {
	return IsFollowing(u.ID, followID)
}

// GetFollowing returns range of user's following.
func (u *User) GetFollowing(page int) ([]*User, error) {
	users := make([]*User, 0, ItemsPerPage)
	sess := x.
		Limit(ItemsPerPage, (page-1)*ItemsPerPage).
		Where("follow.user_id=?", u.ID)
	if setting.UsePostgreSQL {
		sess = sess.Join("LEFT", "follow", `"user".id=follow.follow_id`)
	} else {
		sess = sess.Join("LEFT", "follow", "user.id=follow.follow_id")
	}
	return users, sess.Find(&users)
}

// NewGitSig generates and returns the signature of given user.
func (u *User) NewGitSig() *git.Signature {
	return &git.Signature{
		Name:  u.DisplayName(),
		Email: u.getEmail(),
		When:  time.Now(),
	}
}

// EncodePasswd encodes password to safe format.
func (u *User) EncodePasswd() {
	newPasswd := pbkdf2.Key([]byte(u.Passwd), []byte(u.Salt), 10000, 50, sha256.New)
	u.Passwd = fmt.Sprintf("%x", newPasswd)
}

// ValidatePassword checks if given password matches the one belongs to the user.
func (u *User) ValidatePassword(passwd string) bool {
	newUser := &User{Passwd: passwd, Salt: u.Salt}
	newUser.EncodePasswd()
	return subtle.ConstantTimeCompare([]byte(u.Passwd), []byte(newUser.Passwd)) == 1
}

// IsPasswordSet checks if the password is set or left empty
func (u *User) IsPasswordSet() bool {
	return !u.ValidatePassword("")
}

// UploadAvatar saves custom avatar for user.
// FIXME: split uploads to different subdirs in case we have massive users.
func (u *User) UploadAvatar(data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("Decode: %v", err)
	}

	m := resize.Resize(avatar.AvatarSize, avatar.AvatarSize, img, resize.NearestNeighbor)

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	u.UseCustomAvatar = true
	u.Avatar = fmt.Sprintf("%x", md5.Sum(data))
	if err = updateUser(sess, u); err != nil {
		return fmt.Errorf("updateUser: %v", err)
	}

	if err := os.MkdirAll(setting.AvatarUploadPath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", setting.AvatarUploadPath, err)
	}

	fw, err := os.Create(u.CustomAvatarPath())
	if err != nil {
		return fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()

	if err = png.Encode(fw, m); err != nil {
		return fmt.Errorf("Encode: %v", err)
	}

	return sess.Commit()
}

// DeleteAvatar deletes the user's custom avatar.
func (u *User) DeleteAvatar() error {
	log.Trace("DeleteAvatar[%d]: %s", u.ID, u.CustomAvatarPath())
	if len(u.Avatar) > 0 {
		if err := os.Remove(u.CustomAvatarPath()); err != nil {
			return fmt.Errorf("Failed to remove %s: %v", u.CustomAvatarPath(), err)
		}
	}

	u.UseCustomAvatar = false
	u.Avatar = ""
	if _, err := x.Id(u.ID).Cols("avatar, use_custom_avatar").Update(u); err != nil {
		return fmt.Errorf("UpdateUser: %v", err)
	}
	return nil
}

// IsAdminOfRepo returns true if user has admin or higher access of repository.
func (u *User) IsAdminOfRepo(repo *Repository) bool {
	has, err := HasAccess(u.ID, repo, AccessModeAdmin)
	if err != nil {
		log.Error(3, "HasAccess: %v", err)
	}
	return has
}

// IsWriterOfRepo returns true if user has write access to given repository.
func (u *User) IsWriterOfRepo(repo *Repository) bool {
	has, err := HasAccess(u.ID, repo, AccessModeWrite)
	if err != nil {
		log.Error(3, "HasAccess: %v", err)
	}
	return has
}

// IsOrganization returns true if user is actually a organization.
func (u *User) IsOrganization() bool {
	return u.Type == UserTypeOrganization
}

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func (u *User) IsUserOrgOwner(orgID int64) bool {
	return IsOrganizationOwner(orgID, u.ID)
}

// IsPublicMember returns true if user public his/her membership in given organization.
func (u *User) IsPublicMember(orgID int64) bool {
	return IsPublicMembership(orgID, u.ID)
}

func (u *User) getOrganizationCount(e Engine) (int64, error) {
	return e.
		Where("uid=?", u.ID).
		Count(new(OrgUser))
}

// GetOrganizationCount returns count of membership of organization of user.
func (u *User) GetOrganizationCount() (int64, error) {
	return u.getOrganizationCount(x)
}

// GetRepositories returns repositories that user owns, including private repositories.
func (u *User) GetRepositories(page, pageSize int) (err error) {
	u.Repos, err = GetUserRepositories(u.ID, true, page, pageSize, "")
	return err
}

// GetRepositoryIDs returns repositories IDs where user owned
func (u *User) GetRepositoryIDs() ([]int64, error) {
	var ids []int64
	return ids, x.Table("repository").Cols("id").Where("owner_id = ?", u.ID).Find(&ids)
}

// GetOrgRepositoryIDs returns repositories IDs where user's team owned
func (u *User) GetOrgRepositoryIDs() ([]int64, error) {
	var ids []int64
	return ids, x.Table("repository").
		Cols("repository.id").
		Join("INNER", "team_user", "repository.owner_id = team_user.org_id AND team_user.uid = ?", u.ID).
		GroupBy("repository.id").Find(&ids)
}

// GetAccessRepoIDs returns all repositories IDs where user's or user is a team member organizations
func (u *User) GetAccessRepoIDs() ([]int64, error) {
	ids, err := u.GetRepositoryIDs()
	if err != nil {
		return nil, err
	}
	ids2, err := u.GetOrgRepositoryIDs()
	if err != nil {
		return nil, err
	}
	return append(ids, ids2...), nil
}

// GetMirrorRepositories returns mirror repositories that user owns, including private repositories.
func (u *User) GetMirrorRepositories() ([]*Repository, error) {
	return GetUserMirrorRepositories(u.ID)
}

// GetOwnedOrganizations returns all organizations that user owns.
func (u *User) GetOwnedOrganizations() (err error) {
	u.OwnedOrgs, err = GetOwnedOrgsByUserID(u.ID)
	return err
}

// GetOrganizations returns all organizations that user belongs to.
func (u *User) GetOrganizations(all bool) error {
	ous, err := GetOrgUsersByUserID(u.ID, all)
	if err != nil {
		return err
	}

	u.Orgs = make([]*User, len(ous))
	for i, ou := range ous {
		u.Orgs[i], err = GetUserByID(ou.OrgID)
		if err != nil {
			return err
		}
	}
	return nil
}

// DisplayName returns full name if it's not empty,
// returns username otherwise.
func (u *User) DisplayName() string {
	if len(u.FullName) > 0 {
		return u.FullName
	}
	return u.Name
}

// ShortName ellipses username to length
func (u *User) ShortName(length int) string {
	return base.EllipsisString(u.Name, length)
}

// IsMailable checks if a user is eligible
// to receive emails.
func (u *User) IsMailable() bool {
	return u.IsActive
}

// IsUserExist checks if given user name exist,
// the user name should be noncased unique.
// If uid is presented, then check will rule out that one,
// it is used when update a user name in settings page.
func IsUserExist(uid int64, name string) (bool, error) {
	if len(name) == 0 {
		return false, nil
	}
	return x.
		Where("id!=?", uid).
		Get(&User{LowerName: strings.ToLower(name)})
}

// GetUserSalt returns a random user salt token.
func GetUserSalt() (string, error) {
	return base.GetRandomString(10)
}

// NewGhostUser creates and returns a fake user for someone has deleted his/her account.
func NewGhostUser() *User {
	return &User{
		ID:        -1,
		Name:      "Ghost",
		LowerName: "ghost",
	}
}

var (
	reservedUsernames    = []string{"assets", "css", "explore", "img", "js", "less", "plugins", "debug", "raw", "install", "api", "avatar", "user", "org", "help", "stars", "issues", "pulls", "commits", "repo", "template", "admin", "new", ".", ".."}
	reservedUserPatterns = []string{"*.keys"}
)

// isUsableName checks if name is reserved or pattern of name is not allowed
// based on given reserved names and patterns.
// Names are exact match, patterns can be prefix or suffix match with placeholder '*'.
func isUsableName(names, patterns []string, name string) error {
	name = strings.TrimSpace(strings.ToLower(name))
	if utf8.RuneCountInString(name) == 0 {
		return ErrNameEmpty
	}

	for i := range names {
		if name == names[i] {
			return ErrNameReserved{name}
		}
	}

	for _, pat := range patterns {
		if pat[0] == '*' && strings.HasSuffix(name, pat[1:]) ||
			(pat[len(pat)-1] == '*' && strings.HasPrefix(name, pat[:len(pat)-1])) {
			return ErrNamePatternNotAllowed{pat}
		}
	}

	return nil
}

// IsUsableUsername returns an error when a username is reserved
func IsUsableUsername(name string) error {
	return isUsableName(reservedUsernames, reservedUserPatterns, name)
}

// CreateUser creates record of a new user.
func CreateUser(u *User) (err error) {
	if err = IsUsableUsername(u.Name); err != nil {
		return err
	}

	isExist, err := IsUserExist(0, u.Name)
	if err != nil {
		return err
	} else if isExist {
		return ErrUserAlreadyExist{u.Name}
	}

	u.Email = strings.ToLower(u.Email)
	has, err := x.
		Where("email=?", u.Email).
		Get(new(User))
	if err != nil {
		return err
	} else if has {
		return ErrEmailAlreadyUsed{u.Email}
	}

	isExist, err = IsEmailUsed(u.Email)
	if err != nil {
		return err
	} else if isExist {
		return ErrEmailAlreadyUsed{u.Email}
	}

	u.KeepEmailPrivate = setting.Service.DefaultKeepEmailPrivate

	u.LowerName = strings.ToLower(u.Name)
	u.AvatarEmail = u.Email
	u.Avatar = base.HashEmail(u.AvatarEmail)
	if u.Rands, err = GetUserSalt(); err != nil {
		return err
	}
	if u.Salt, err = GetUserSalt(); err != nil {
		return err
	}
	u.EncodePasswd()
	u.AllowCreateOrganization = setting.Service.DefaultAllowCreateOrganization
	u.MaxRepoCreation = -1

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(u); err != nil {
		return err
	} else if err = os.MkdirAll(UserPath(u.Name), os.ModePerm); err != nil {
		return err
	}

	return sess.Commit()
}

func countUsers(e Engine) int64 {
	count, _ := e.
		Where("type=0").
		Count(new(User))
	return count
}

// CountUsers returns number of users.
func CountUsers() int64 {
	return countUsers(x)
}

// Users returns number of users in given page.
func Users(opts *SearchUserOptions) ([]*User, error) {
	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "name ASC"
	}

	users := make([]*User, 0, opts.PageSize)
	sess := x.
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		Where("type=0")

	return users, sess.
		OrderBy(opts.OrderBy).
		Find(&users)
}

// get user by verify code
func getVerifyUser(code string) (user *User) {
	if len(code) <= base.TimeLimitCodeLength {
		return nil
	}

	// use tail hex username query user
	hexStr := code[base.TimeLimitCodeLength:]
	if b, err := hex.DecodeString(hexStr); err == nil {
		if user, err = GetUserByName(string(b)); user != nil {
			return user
		}
		log.Error(4, "user.getVerifyUser: %v", err)
	}

	return nil
}

// VerifyUserActiveCode verifies active code when active account
func VerifyUserActiveCode(code string) (user *User) {
	minutes := setting.Service.ActiveCodeLives

	if user = getVerifyUser(code); user != nil {
		// time limit code
		prefix := code[:base.TimeLimitCodeLength]
		data := com.ToStr(user.ID) + user.Email + user.LowerName + user.Passwd + user.Rands

		if base.VerifyTimeLimitCode(data, minutes, prefix) {
			return user
		}
	}
	return nil
}

// VerifyActiveEmailCode verifies active email code when active account
func VerifyActiveEmailCode(code, email string) *EmailAddress {
	minutes := setting.Service.ActiveCodeLives

	if user := getVerifyUser(code); user != nil {
		// time limit code
		prefix := code[:base.TimeLimitCodeLength]
		data := com.ToStr(user.ID) + email + user.LowerName + user.Passwd + user.Rands

		if base.VerifyTimeLimitCode(data, minutes, prefix) {
			emailAddress := &EmailAddress{Email: email}
			if has, _ := x.Get(emailAddress); has {
				return emailAddress
			}
		}
	}
	return nil
}

// ChangeUserName changes all corresponding setting from old user name to new one.
func ChangeUserName(u *User, newUserName string) (err error) {
	if err = IsUsableUsername(newUserName); err != nil {
		return err
	}

	isExist, err := IsUserExist(0, newUserName)
	if err != nil {
		return err
	} else if isExist {
		return ErrUserAlreadyExist{newUserName}
	}

	if err = ChangeUsernameInPullRequests(u.Name, newUserName); err != nil {
		return fmt.Errorf("ChangeUsernameInPullRequests: %v", err)
	}

	// Delete all local copies of repository wiki that user owns.
	if err = x.
		Where("owner_id=?", u.ID).
		Iterate(new(Repository), func(idx int, bean interface{}) error {
			repo := bean.(*Repository)
			RemoveAllWithNotice("Delete repository wiki local copy", repo.LocalWikiPath())
			return nil
		}); err != nil {
		return fmt.Errorf("Delete repository wiki local copy: %v", err)
	}

	return os.Rename(UserPath(u.Name), UserPath(newUserName))
}

// checkDupEmail checks whether there are the same email with the user
func checkDupEmail(e Engine, u *User) error {
	u.Email = strings.ToLower(u.Email)
	has, err := e.
		Where("id!=?", u.ID).
		And("type=?", u.Type).
		And("email=?", u.Email).
		Get(new(User))
	if err != nil {
		return err
	} else if has {
		return ErrEmailAlreadyUsed{u.Email}
	}
	return nil
}

func updateUser(e Engine, u *User) error {
	// Organization does not need email
	u.Email = strings.ToLower(u.Email)
	if !u.IsOrganization() {
		if len(u.AvatarEmail) == 0 {
			u.AvatarEmail = u.Email
		}
		if len(u.AvatarEmail) > 0 {
			u.Avatar = base.HashEmail(u.AvatarEmail)
		}
	}

	u.LowerName = strings.ToLower(u.Name)
	u.Location = base.TruncateString(u.Location, 255)
	u.Website = base.TruncateString(u.Website, 255)
	u.Description = base.TruncateString(u.Description, 255)

	_, err := e.Id(u.ID).AllCols().Update(u)
	return err
}

// UpdateUser updates user's information.
func UpdateUser(u *User) error {
	return updateUser(x, u)
}

// UpdateUserCols update user according special columns
func UpdateUserCols(u *User, cols ...string) error {
	// Organization does not need email
	u.Email = strings.ToLower(u.Email)
	if !u.IsOrganization() {
		if len(u.AvatarEmail) == 0 {
			u.AvatarEmail = u.Email
		}
		if len(u.AvatarEmail) > 0 {
			u.Avatar = base.HashEmail(u.AvatarEmail)
		}
	}

	u.LowerName = strings.ToLower(u.Name)
	u.Location = base.TruncateString(u.Location, 255)
	u.Website = base.TruncateString(u.Website, 255)
	u.Description = base.TruncateString(u.Description, 255)

	_, err := x.Id(u.ID).Cols(cols...).Update(u)
	return err
}

// UpdateUserSetting updates user's settings.
func UpdateUserSetting(u *User) error {
	if !u.IsOrganization() {
		if err := checkDupEmail(x, u); err != nil {
			return err
		}
	}
	return updateUser(x, u)
}

// deleteBeans deletes all given beans, beans should contain delete conditions.
func deleteBeans(e Engine, beans ...interface{}) (err error) {
	for i := range beans {
		if _, err = e.Delete(beans[i]); err != nil {
			return err
		}
	}
	return nil
}

// FIXME: need some kind of mechanism to record failure. HINT: system notice
func deleteUser(e *xorm.Session, u *User) error {
	// Note: A user owns any repository or belongs to any organization
	//	cannot perform delete operation.

	// Check ownership of repository.
	count, err := getRepositoryCount(e, u)
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %v", err)
	} else if count > 0 {
		return ErrUserOwnRepos{UID: u.ID}
	}

	// Check membership of organization.
	count, err = u.getOrganizationCount(e)
	if err != nil {
		return fmt.Errorf("GetOrganizationCount: %v", err)
	} else if count > 0 {
		return ErrUserHasOrgs{UID: u.ID}
	}

	// ***** START: Watch *****
	watchedRepoIDs := make([]int64, 0, 10)
	if err = e.Table("watch").Cols("watch.repo_id").
		Where("watch.user_id = ?", u.ID).Find(&watchedRepoIDs); err != nil {
		return fmt.Errorf("get all watches: %v", err)
	}
	if _, err = e.Decr("num_watches").In("id", watchedRepoIDs).Update(new(Repository)); err != nil {
		return fmt.Errorf("decrease repository num_watches: %v", err)
	}
	// ***** END: Watch *****

	// ***** START: Star *****
	starredRepoIDs := make([]int64, 0, 10)
	if err = e.Table("star").Cols("star.repo_id").
		Where("star.uid = ?", u.ID).Find(&starredRepoIDs); err != nil {
		return fmt.Errorf("get all stars: %v", err)
	} else if _, err = e.Decr("num_watches").In("id", starredRepoIDs).Update(new(Repository)); err != nil {
		return fmt.Errorf("decrease repository num_stars: %v", err)
	}
	// ***** END: Star *****

	// ***** START: Follow *****
	followeeIDs := make([]int64, 0, 10)
	if err = e.Table("follow").Cols("follow.follow_id").
		Where("follow.user_id = ?", u.ID).Find(&followeeIDs); err != nil {
		return fmt.Errorf("get all followees: %v", err)
	} else if _, err = e.Decr("num_followers").In("id", followeeIDs).Update(new(User)); err != nil {
		return fmt.Errorf("decrease user num_followers: %v", err)
	}

	followerIDs := make([]int64, 0, 10)
	if err = e.Table("follow").Cols("follow.user_id").
		Where("follow.follow_id = ?", u.ID).Find(&followerIDs); err != nil {
		return fmt.Errorf("get all followers: %v", err)
	} else if _, err = e.Decr("num_following").In("id", followerIDs).Update(new(User)); err != nil {
		return fmt.Errorf("decrease user num_following: %v", err)
	}
	// ***** END: Follow *****

	if err = deleteBeans(e,
		&AccessToken{UID: u.ID},
		&Collaboration{UserID: u.ID},
		&Access{UserID: u.ID},
		&Watch{UserID: u.ID},
		&Star{UID: u.ID},
		&Follow{UserID: u.ID},
		&Follow{FollowID: u.ID},
		&Action{UserID: u.ID},
		&IssueUser{UID: u.ID},
		&EmailAddress{UID: u.ID},
		&UserOpenID{UID: u.ID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	// ***** START: PublicKey *****
	keys := make([]*PublicKey, 0, 10)
	if err = e.Find(&keys, &PublicKey{OwnerID: u.ID}); err != nil {
		return fmt.Errorf("get all public keys: %v", err)
	}

	keyIDs := make([]int64, len(keys))
	for i := range keys {
		keyIDs[i] = keys[i].ID
	}
	if err = deletePublicKeys(e, keyIDs...); err != nil {
		return fmt.Errorf("deletePublicKeys: %v", err)
	}
	// ***** END: PublicKey *****

	// Clear assignee.
	if _, err = e.Exec("UPDATE `issue` SET assignee_id=0 WHERE assignee_id=?", u.ID); err != nil {
		return fmt.Errorf("clear assignee: %v", err)
	}

	// ***** START: ExternalLoginUser *****
	if err = removeAllAccountLinks(e, u); err != nil {
		return fmt.Errorf("ExternalLoginUser: %v", err)
	}
	// ***** END: ExternalLoginUser *****

	if _, err = e.Id(u.ID).Delete(new(User)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	// FIXME: system notice
	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := UserPath(u.Name)

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("Failed to RemoveAll %s: %v", path, err)
	}

	if len(u.Avatar) > 0 {
		avatarPath := u.CustomAvatarPath()
		if com.IsExist(avatarPath) {
			if err := os.Remove(avatarPath); err != nil {
				return fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			}
		}
	}

	return nil
}

// DeleteUser completely and permanently deletes everything of a user,
// but issues/comments/pulls will be kept and shown as someone has been deleted.
func DeleteUser(u *User) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deleteUser(sess, u); err != nil {
		// Note: don't wrapper error here.
		return err
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	return RewriteAllPublicKeys()
}

// DeleteInactivateUsers deletes all inactivate users and email addresses.
func DeleteInactivateUsers() (err error) {
	users := make([]*User, 0, 10)
	if err = x.
		Where("is_active = ?", false).
		Find(&users); err != nil {
		return fmt.Errorf("get all inactive users: %v", err)
	}
	// FIXME: should only update authorized_keys file once after all deletions.
	for _, u := range users {
		if err = DeleteUser(u); err != nil {
			// Ignore users that were set inactive by admin.
			if IsErrUserOwnRepos(err) || IsErrUserHasOrgs(err) {
				continue
			}
			return err
		}
	}

	_, err = x.
		Where("is_activated = ?", false).
		Delete(new(EmailAddress))
	return err
}

// UserPath returns the path absolute path of user repositories.
func UserPath(userName string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(userName))
}

// GetUserByKeyID get user information by user's public key id
func GetUserByKeyID(keyID int64) (*User, error) {
	var user User
	has, err := x.Join("INNER", "public_key", "`public_key`.owner_id = `user`.id").
		Where("`public_key`.id=?", keyID).
		Get(&user)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrUserNotExist{0, "", keyID}
	}
	return &user, nil
}

func getUserByID(e Engine, id int64) (*User, error) {
	u := new(User)
	has, err := e.Id(id).Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserNotExist{id, "", 0}
	}
	return u, nil
}

// GetUserByID returns the user object by given ID if exists.
func GetUserByID(id int64) (*User, error) {
	return getUserByID(x, id)
}

// GetAssigneeByID returns the user with write access of repository by given ID.
func GetAssigneeByID(repo *Repository, userID int64) (*User, error) {
	has, err := HasAccess(userID, repo, AccessModeWrite)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserNotExist{userID, "", 0}
	}
	return GetUserByID(userID)
}

// GetUserByName returns user by given name.
func GetUserByName(name string) (*User, error) {
	return getUserByName(x, name)
}

func getUserByName(e Engine, name string) (*User, error) {
	if len(name) == 0 {
		return nil, ErrUserNotExist{0, name, 0}
	}
	u := &User{LowerName: strings.ToLower(name)}
	has, err := e.Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserNotExist{0, name, 0}
	}
	return u, nil
}

// GetUserEmailsByNames returns a list of e-mails corresponds to names.
func GetUserEmailsByNames(names []string) []string {
	return getUserEmailsByNames(x, names)
}

func getUserEmailsByNames(e Engine, names []string) []string {
	mails := make([]string, 0, len(names))
	for _, name := range names {
		u, err := getUserByName(e, name)
		if err != nil {
			continue
		}
		if u.IsMailable() {
			mails = append(mails, u.Email)
		}
	}
	return mails
}

// GetUsersByIDs returns all resolved users from a list of Ids.
func GetUsersByIDs(ids []int64) ([]*User, error) {
	ous := make([]*User, 0, len(ids))
	if len(ids) == 0 {
		return ous, nil
	}
	err := x.In("id", ids).
		Asc("name").
		Find(&ous)
	return ous, err
}

// GetUserIDsByNames returns a slice of ids corresponds to names.
func GetUserIDsByNames(names []string) []int64 {
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		u, err := GetUserByName(name)
		if err != nil {
			continue
		}
		ids = append(ids, u.ID)
	}
	return ids
}

// UserCommit represents a commit with validation of user.
type UserCommit struct {
	User *User
	*git.Commit
}

// ValidateCommitWithEmail check if author's e-mail of commit is corresponding to a user.
func ValidateCommitWithEmail(c *git.Commit) *User {
	u, err := GetUserByEmail(c.Author.Email)
	if err != nil {
		return nil
	}
	return u
}

// ValidateCommitsWithEmails checks if authors' e-mails of commits are corresponding to users.
func ValidateCommitsWithEmails(oldCommits *list.List) *list.List {
	var (
		u          *User
		emails     = map[string]*User{}
		newCommits = list.New()
		e          = oldCommits.Front()
	)
	for e != nil {
		c := e.Value.(*git.Commit)

		if v, ok := emails[c.Author.Email]; !ok {
			u, _ = GetUserByEmail(c.Author.Email)
			emails[c.Author.Email] = u
		} else {
			u = v
		}

		newCommits.PushBack(UserCommit{
			User:   u,
			Commit: c,
		})
		e = e.Next()
	}
	return newCommits
}

// GetUserByEmail returns the user object by given e-mail if exists.
func GetUserByEmail(email string) (*User, error) {
	if len(email) == 0 {
		return nil, ErrUserNotExist{0, email, 0}
	}

	email = strings.ToLower(email)
	// First try to find the user by primary email
	user := &User{Email: email}
	has, err := x.Get(user)
	if err != nil {
		return nil, err
	}
	if has {
		return user, nil
	}

	// Otherwise, check in alternative list for activated email addresses
	emailAddress := &EmailAddress{Email: email, IsActivated: true}
	has, err = x.Get(emailAddress)
	if err != nil {
		return nil, err
	}
	if has {
		return GetUserByID(emailAddress.UID)
	}

	return nil, ErrUserNotExist{0, email, 0}
}

// GetUser checks if a user already exists
func GetUser(user *User) (bool, error) {
	return x.Get(user)
}

// SearchUserOptions contains the options for searching
type SearchUserOptions struct {
	Keyword  string
	Type     UserType
	OrderBy  string
	Page     int
	PageSize int // Can be smaller than or equal to setting.UI.ExplorePagingNum
}

// SearchUserByName takes keyword and part of user name to search,
// it returns results in given range and number of total results.
func SearchUserByName(opts *SearchUserOptions) (users []*User, _ int64, _ error) {
	if len(opts.Keyword) == 0 {
		return users, 0, nil
	}
	opts.Keyword = strings.ToLower(opts.Keyword)

	if opts.PageSize <= 0 || opts.PageSize > setting.UI.ExplorePagingNum {
		opts.PageSize = setting.UI.ExplorePagingNum
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}

	users = make([]*User, 0, opts.PageSize)

	// Append conditions
	cond := builder.And(
		builder.Eq{"type": opts.Type},
		builder.Or(
			builder.Like{"lower_name", opts.Keyword},
			builder.Like{"LOWER(full_name)", opts.Keyword},
		),
	)

	count, err := x.Where(cond).Count(new(User))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	sess := x.Where(cond).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	if len(opts.OrderBy) > 0 {
		sess.OrderBy(opts.OrderBy)
	}
	return users, count, sess.Find(&users)
}

// GetStarredRepos returns the repos starred by a particular user
func GetStarredRepos(userID int64, private bool) ([]*Repository, error) {
	sess := x.Where("star.uid=?", userID).
		Join("LEFT", "star", "`repository`.id=`star`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}
	repos := make([]*Repository, 0, 10)
	err := sess.Find(&repos)
	if err != nil {
		return nil, err
	}
	return repos, nil
}

// GetWatchedRepos returns the repos watched by a particular user
func GetWatchedRepos(userID int64, private bool) ([]*Repository, error) {
	sess := x.Where("watch.user_id=?", userID).
		Join("LEFT", "watch", "`repository`.id=`watch`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}
	repos := make([]*Repository, 0, 10)
	err := sess.Find(&repos)
	if err != nil {
		return nil, err
	}
	return repos, nil
}

// SyncExternalUsers is used to synchronize users with external authorization source
func SyncExternalUsers() {
	if !taskStatusTable.StartIfNotRunning(syncExternalUsers) {
		return
	}
	defer taskStatusTable.Stop(syncExternalUsers)

	log.Trace("Doing: SyncExternalUsers")

	ls, err := LoginSources()
	if err != nil {
		log.Error(4, "SyncExternalUsers: %v", err)
		return
	}

	updateExisting := setting.Cron.SyncExternalUsers.UpdateExisting

	for _, s := range ls {
		if !s.IsActived || !s.IsSyncEnabled {
			continue
		}
		if s.IsLDAP() {
			log.Trace("Doing: SyncExternalUsers[%s]", s.Name)

			var existingUsers []int64

			// Find all users with this login type
			var users []User
			x.Where("login_type = ?", LoginLDAP).
				And("login_source = ?", s.ID).
				Find(&users)

			sr := s.LDAP().SearchEntries()

			for _, su := range sr {
				if len(su.Username) == 0 {
					continue
				}

				if len(su.Mail) == 0 {
					su.Mail = fmt.Sprintf("%s@localhost", su.Username)
				}

				var usr *User
				// Search for existing user
				for _, du := range users {
					if du.LowerName == strings.ToLower(su.Username) {
						usr = &du
						break
					}
				}

				fullName := composeFullName(su.Name, su.Surname, su.Username)
				// If no existing user found, create one
				if usr == nil {
					log.Trace("SyncExternalUsers[%s]: Creating user %s", s.Name, su.Username)

					usr = &User{
						LowerName:   strings.ToLower(su.Username),
						Name:        su.Username,
						FullName:    fullName,
						LoginType:   s.Type,
						LoginSource: s.ID,
						LoginName:   su.Username,
						Email:       su.Mail,
						IsAdmin:     su.IsAdmin,
						IsActive:    true,
					}

					err = CreateUser(usr)
					if err != nil {
						log.Error(4, "SyncExternalUsers[%s]: Error creating user %s: %v", s.Name, su.Username, err)
					}
				} else if updateExisting {
					existingUsers = append(existingUsers, usr.ID)
					// Check if user data has changed
					if (len(s.LDAP().AdminFilter) > 0 && usr.IsAdmin != su.IsAdmin) ||
						strings.ToLower(usr.Email) != strings.ToLower(su.Mail) ||
						usr.FullName != fullName ||
						!usr.IsActive {

						log.Trace("SyncExternalUsers[%s]: Updating user %s", s.Name, usr.Name)

						usr.FullName = fullName
						usr.Email = su.Mail
						// Change existing admin flag only if AdminFilter option is set
						if len(s.LDAP().AdminFilter) > 0 {
							usr.IsAdmin = su.IsAdmin
						}
						usr.IsActive = true

						err = UpdateUserCols(usr, "full_name", "email", "is_admin", "is_active")
						if err != nil {
							log.Error(4, "SyncExternalUsers[%s]: Error updating user %s: %v", s.Name, usr.Name, err)
						}
					}
				}
			}

			// Deactivate users not present in LDAP
			if updateExisting {
				for _, usr := range users {
					found := false
					for _, uid := range existingUsers {
						if usr.ID == uid {
							found = true
							break
						}
					}
					if !found {
						log.Trace("SyncExternalUsers[%s]: Deactivating user %s", s.Name, usr.Name)

						usr.IsActive = false
						err = UpdateUserCols(&usr, "is_active")
						if err != nil {
							log.Error(4, "SyncExternalUsers[%s]: Error deactivating user %s: %v", s.Name, usr.Name, err)
						}
					}
				}
			}
		}
	}
}
