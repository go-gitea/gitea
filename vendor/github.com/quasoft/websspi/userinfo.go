package websspi

// UserInfo represents an authenticated user.
type UserInfo struct {
	Username string   // Name of user, usually in the form DOMAIN\User
	Groups   []string // The global groups the user is a member of
}
