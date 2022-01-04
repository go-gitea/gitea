package auth

import (
	"fmt"
	"net/url"

	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/dgrijalva/jwt-go"
)

func TrapSignIn(ctx *context.Context) {
	// Check auto-login.
	if checkAutoLogin(ctx) {
		return
	}

	token := ctx.GetCookie("traP_token")
	if user := getUserFromTrapToken(token); user != nil {
		handleSignIn(ctx, user, false)
	} else {
		ctx.Redirect("https://portal.trap.jp/pipeline?redirect=" + url.QueryEscape(setting.AppURL+"user/login"))
	}
}

func getUserFromTrapToken(tokenString string) *user.User {
	if tokenString == "" {
		log.Warn("No token")
		return nil
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		pubKey, _ := jwt.ParseRSAPublicKeyFromPEM(pubKeyPEM)
		return pubKey, nil
	})

	if err != nil || !token.Valid {
		log.Warn("Failed to parse token: %v", err)
		return nil
	}

	data := token.Claims.(jwt.MapClaims)
	log.Debug("traP token accepted: %s", data["id"])

	u, _ := user.GetUserByName(data["id"].(string))
	if u == nil {
		u = &user.User{
			Name:     data["id"].(string),
			Email:    data["email"].(string),
			Passwd:   "",
			IsActive: true,
		}
		if err := user.CreateUser(u); err != nil {
			log.ErrorWithSkip(3, "Failed to create account: %v", err)
			return nil
		}
		log.Trace("Account created: %s", u.Name)
	}

	u.Email = data["email"].(string)
	u.FullName = data["firstName"].(string) + " " + data["lastName"].(string)
	u.SetLastLogin()

	if err := user.UpdateUser(u); err != nil {
		log.ErrorWithSkip(3, "Failed to update user: %v", err)
		return nil
	}

	return u
}

var pubKeyPEM []byte = []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAraewUw7V1hiuSgUvkly9
X+tcIh0e/KKqeFnAo8WR3ez2tA0fGwM+P8sYKHIDQFX7ER0c+ecTiKpo/Zt/a6AO
gB/zHb8L4TWMr2G4q79S1gNw465/SEaGKR8hRkdnxJ6LXdDEhgrH2ZwIPzE0EVO1
eFrDms1jS3/QEyZCJ72oYbAErI85qJDF/y/iRgl04XBK6GLIW11gpf8KRRAh4vuh
g5/YhsWUdcX+uDVthEEEGOikSacKZMFGZNi8X8YVnRyWLf24QTJnTHEv+0EStNrH
HnxCPX0m79p7tBfFC2ha2OYfOtA+94ZfpZXUi2r6gJZ+dq9FWYyA0DkiYPUq9QMb
OQIDAQAB
-----END PUBLIC KEY-----`)
