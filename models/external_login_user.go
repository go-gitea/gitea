package models

import "github.com/markbates/goth"

// ExternalLoginUser makes the connecting between some existing user and additional external login sources
type ExternalLoginUser struct {
	ExternalID    string `xorm:"NOT NULL"`
	UserID        int64 `xorm:"NOT NULL"`
	LoginSourceID int64 `xorm:"NOT NULL"`
}

// GetExternalLogin checks if a externalID in loginSourceID scope already exists
func GetExternalLogin(externalLoginUser *ExternalLoginUser) (bool, error) {
	return x.Get(externalLoginUser)
}

// LinkAccountToUser link the gothUser to the user
func LinkAccountToUser(user *User, gothUser goth.User) error {
	loginSource, err := GetActiveOAuth2LoginSourceByName(gothUser.Provider)
	if err != nil {
		return err
	}

	externalLoginUser := &ExternalLoginUser{
		ExternalID:    gothUser.UserID,
		UserID:        user.ID,
		LoginSourceID: loginSource.ID,
	}
	has, err := x.Get(externalLoginUser)
	if err != nil {
		return err
	} else if has {
		return ErrExternalLoginUserAlreadyExist{gothUser.UserID, user.ID, loginSource.ID}
	}

	_, err = x.Insert(externalLoginUser)
	return err
}

// RemoveAllAccountLinks will remove all external login sources for the given user
func RemoveAllAccountLinks(user *User) error {
	_, err := x.Delete(&ExternalLoginUser{UserID: user.ID})
	return err
}