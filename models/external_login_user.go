package models

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
