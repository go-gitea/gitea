// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwk"
)

type accessToken struct {
	Sub      string `json:"sub"`
	EventID  string `json:"event_id"`
	TokenUse string `json:"token_use"`
	Scope    string `json:"scope"`
	AuthTime int    `json:"auth_time"`
	Iss      string `json:"iss"`
	//Exp      time.Time    `json:"exp"`
	Iat      int    `json:"iat"`
	Jti      string `json:"jti"`
	ClientID string `json:"client_id"`
	Username string `json:"username"`
	jwt.StandardClaims
}

type JWK struct {
	Keys []struct {
		Alg string `json:"alg"`
		E   string `json:"e"`
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
	} `json:"keys"`
}

// type key struct {
// 	Alg string `json:"alg"`
// 	E   string `json:"e"`
// 	Kid string `json:"kid"`
// 	Kty string `json:"kty"`
// 	N   string `json:"n"`
// 	Use string `json:"use"`
// }
// type Keys struct {
// 	Keys []key
// }

// Ensure the struct implements the interface.
var (
	_ Method = &OAuth2{}
	_ Named  = &OAuth2{}
)

// CheckOAuthAccessToken returns uid of user from oauth token
func CheckOAuthAccessToken(accessToken string) int64 {
	// JWT tokens require a "."
	if !strings.Contains(accessToken, ".") {
		return 0
	}
	token, err := oauth2.ParseToken(accessToken, oauth2.DefaultSigningKey)
	if err != nil {
		log.Trace("oauth2.ParseToken: %v", err)
		return 0
	}
	var grant *auth_model.OAuth2Grant
	if grant, err = auth_model.GetOAuth2GrantByID(db.DefaultContext, token.GrantID); err != nil || grant == nil {
		return 0
	}
	if token.Type != oauth2.TypeAccessToken {
		return 0
	}
	if token.ExpiresAt.Before(time.Now()) || token.IssuedAt.After(time.Now()) {
		return 0
	}
	return grant.UserID
}

// GetPublicKey function to get the public key from json file
func GetPublicKey(kid string) (interface{}, error) {
	_, err := setting.CheckCognitoSecretFile()
	if os.IsNotExist(err) {
		log.Error("JSON FILE NOT FOUND %s", err)
		// Call Function to Generate the secret json file
		err = GenerateJSONFile()
		if err != nil {
			log.Error("JSON FILE CAN'T CREATED %s", err)
			return nil, err
		}
	}
	// INITIAL STRUCT OF KEY
	keySet := JWK{}
	// Open the secret key file
	jsonFile, err := setting.OpenCognitoSecretFile()
	//check on the error
	if err != nil {
		log.Error("CAN'T OPEN JSON FILE %s", err)
		return nil, err
	}
	//close the file in end of function
	defer jsonFile.Close()
	// load data
	byteValue, err := io.ReadAll(jsonFile)
	//check on the error
	if err != nil {
		log.Error("CAN'T READ DATA FROM JSON FILE %s", err)
		return nil, err
	}
	err = json.Unmarshal(byteValue, &keySet)
	//check on the error
	if err != nil {
		log.Error("UNMARSHAL DATA IN JSON FILE %s", err)
		return nil, err
	}
	// CONVERT DATA  TO BYTES
	date, err := json.Marshal(keySet)
	if err != nil {
		log.Error("MARSHAL DATA IN JSON FILE %s", err)
		return nil, err
	}
	keyset, err := jwk.Parse(date)

	if err != nil {
		log.Error("PARSE DATA IN JSON FILE %s", err)
		return nil, err
	}
	// GET THE KID
	keys, ok := keyset.LookupKeyID(kid)
	if !ok {
		log.Error("Can't load kid from key set  %s", err)

		return nil, fmt.Errorf("can't load kid from key set ")
	}
	var publickey interface{}
	err = keys.Raw(&publickey)
	if err != nil {
		log.Error("Can't load data in public key  %s", err)

		return nil, fmt.Errorf("can't load data in public key ")
	}
	// return the interface as a public key , error
	return publickey, nil
}

// OAuth2 implements the Auth interface and authenticates requests
// (API requests only) by looking for an OAuth token in query parameters or the
// "Authorization" header.
type OAuth2 struct{}

// Name represents the name of auth method
func (o *OAuth2) Name() string {
	return "oauth2"
}

// GenerateJSONFile -- function to create a secret json file
func GenerateJSONFile() error {

	CognitoURL, err := setting.GetAWSKey(setting.SecAWS, setting.KeyAWS)
	if err != nil {
		return err
	}
	// INITIAL STRUCT OF KEY
	keySet := JWK{}
	// HTTP REQ TO GET THE SECRET KEY
	res, err := http.Get(CognitoURL.Value())
	// CHECK ON THE ERROR OF RESPONSE
	if err != nil {
		return fmt.Errorf("can't get the response ")
	}
	// EXTRACT TO THE BODY FROM RESPONSE
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return fmt.Errorf("can't read the response body ")
	}
	// LOAD RES BODY TO THE JSON FORMAT IN STRUCT
	jsonErr := json.Unmarshal(body, &keySet)
	if jsonErr != nil {
		return fmt.Errorf("can't load data in json format ")
	}
	file, err := json.MarshalIndent(keySet, "", " ")
	if err != nil {
		return err
	}
	err = setting.WriteInCognitoSecretFile(file)
	if err != nil {
		return err
	}
	return nil
}

// CheckCognitoAccessToken -- function to check about the cognito acces token
func CheckCognitoAccessToken(tokenSHA string) (*user_model.User, error) {
	// PARSE TO ACCESS TOKEN
	token, err := jwt.ParseWithClaims(tokenSHA, &accessToken{}, func(tokenSHA *jwt.Token) (interface{}, error) {
		kid, ok := tokenSHA.Header["kid"].(string)
		if !ok {
			log.Error("Can't Get KID .")
			return nil, fmt.Errorf("kid header not found")
		}
		return GetPublicKey(kid)
	})
	// check if the any error when they parse the access token
	if err != nil || !token.Valid {
		log.Error("Error When Get Public Key %s ", err)
		return nil, fmt.Errorf("error when get public key %s ", err)
	}
	// EXTRACT DATA FROM ACCESS TOKEN
	claims, ok := token.Claims.(*accessToken)
	if !ok {
		// CAN'T PARSE CLAIMS
		return nil, fmt.Errorf("can't parse claims")

	}

	// CHECK ON THE EXPIRE DATE
	if claims.ExpiresAt < time.Now().UTC().Unix() {
		// ACCESS TOKEN IS EXPIRE !
		return nil, fmt.Errorf("access token is expired ")

	}

	// CHECK IF THE USER ALREADY EXIST
	u, err := user_model.GetUserByName(db.DefaultContext, claims.Username)

	// IF THE USER NOT EXISTS
	if err != nil {
		provider, err := setting.GetAWSKey(setting.SecAWS, setting.KeyClient)
		if err != nil {
			return nil, err
		}
		sess := session.Must(session.NewSession())
		reg := strings.Split(claims.Iss, ".")[1]
		sess.Config.Region = aws.String(reg)
		cognitoClient := cognitoidentityprovider.New(sess)
		res, err := cognitoClient.GetUser(&cognitoidentityprovider.GetUserInput{AccessToken: &tokenSHA})
		if err != nil {
			return nil, fmt.Errorf("ACCESS TOKEN NOT VALID ON COGNITO")

		}

		var emailIndex int
		for i := 0; i < len(res.UserAttributes); i++ {
			if *res.UserAttributes[i].Name == "email" {
				emailIndex = i
			}
		}

		loginSource, err := auth_model.GetActiveOAuth2SourceByName(provider.Value())
		if err != nil {
			log.Debug("%v", err)
			return nil, err
		}
		if loginSource == nil {

			if err := auth_model.CreateSource(&auth_model.Source{
				Type:          auth_model.OAuth2,
				Name:          provider.Value(),
				IsActive:      true,
				IsSyncEnabled: true,
				Cfg:           &oauth2.Source{},
			}); err != nil {
				if models.IsErrLoginSourceAlreadyExist(err) {
					return nil, err.(models.ErrLoginSourceAlreadyExist)
				} else {
					return nil, err
				}
			}
			loginSource, err = auth_model.GetActiveOAuth2SourceByName(provider.Value())
			if err != nil || loginSource == nil {
				return nil, err
			}
		}

		log.Debug("User will be created")
		// CREATE NEW USER
		u = &user_model.User{
			LowerName:   strings.ToLower(*res.Username),
			Name:        *res.Username,
			LoginType:   auth_model.OAuth2,
			LoginSource: loginSource.ID,
			LoginName:   *res.Username,
			Email:       *res.UserAttributes[emailIndex].Value,
			IsActive:    !(setting.Service.RegisterEmailConfirm || setting.Service.RegisterManualConfirm),
		}

		err = user_model.CreateUser(u)
		if err != nil {
			fmt.Println(err)
			// SOME ERROR ACCURE WHEN CREATING USER
			return nil, fmt.Errorf("CAN'T CREATE NEW USER WITH COGNITO ACCES TOKEN %s ", err)
		}
		// Auto-set admin for the only user.
		if user_model.CountUsers(nil) == 1 {
			u.IsAdmin = true
			u.IsActive = true
			u.SetLastLogin()
			if err := user_model.UpdateUserCols(db.DefaultContext, u, "is_admin", "is_active", "last_login_unix"); err != nil {
				return nil, fmt.Errorf("UpdateUser %s ", err)

			}
		}

	}
	return u, nil
}

// userIDFromToken returns the user id corresponding to the OAuth token.
func (o *OAuth2) userIDFromToken(req *http.Request, store DataStore) int64 {
	err := req.ParseForm()
	if err != nil {
		log.Error("Can't parse from form", err)
		return 0
	}
	isCognito := false

	// Check access token.
	tokenSHA := req.Form.Get("token")
	if len(tokenSHA) == 0 {
		tokenSHA = req.Form.Get("access_token")
	}
	if len(tokenSHA) == 0 {
		tokenSHA = req.Header.Get("COGNITO-TOKEN")
		isCognito = true
	}
	if len(tokenSHA) == 0 {
		// Well, check with header again.
		auHead := req.Header.Get("Authorization")
		if len(auHead) > 0 {
			auths := strings.Fields(auHead)
			if len(auths) == 2 && (auths[0] == "token" || strings.ToLower(auths[0]) == "bearer") {
				tokenSHA = auths[1]
			}
		}
	}
	if len(tokenSHA) == 0 {
		return 0
	}

	// WHEN THE TOKEN IS COGNITO ACCESS TOKEN
	if isCognito {
		u, err := CheckCognitoAccessToken(tokenSHA)
		if err != nil {
			log.Error("CheckCognitoAccessToken %v", err)
			return 0
		}
		store.GetData()["IsApiToken"] = true
		return u.ID

	}

	// Let's see if token is valid.
	if strings.Contains(tokenSHA, ".") {
		uid := CheckOAuthAccessToken(tokenSHA)
		if uid != 0 {
			store.GetData()["IsApiToken"] = true
		}
		return uid
	}
	t, err := auth_model.GetAccessTokenBySHA(tokenSHA)
	if err != nil {
		if !auth_model.IsErrAccessTokenNotExist(err) && !auth_model.IsErrAccessTokenEmpty(err) {
			log.Error("GetAccessTokenBySHA: %v", err)
		}
		return 0
	}
	t.UpdatedUnix = timeutil.TimeStampNow()
	if err = auth_model.UpdateAccessToken(t); err != nil {
		log.Error("UpdateAccessToken: %v", err)
	}
	store.GetData()["IsApiToken"] = true
	return t.UID
}

// Verify extracts the user ID from the OAuth token in the query parameters
// or the "Authorization" header and returns the corresponding user object for that ID.
// If verification is successful returns an existing user object.
// Returns nil if verification fails.
func (o *OAuth2) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *user_model.User {
	if !db.HasEngine {
		return nil
	}

	if !middleware.IsAPIPath(req) && !isAttachmentDownload(req) && !isAuthenticatedTokenRequest(req) {
		return nil
	}

	id := o.userIDFromToken(req, store)
	if id <= 0 {
		return nil
	}
	log.Trace("OAuth2 Authorization: Found token for user[%d]", id)

	user, err := user_model.GetUserByID(id)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
		}
		return nil
	}

	log.Trace("OAuth2 Authorization: Logged in user %-v", user)
	return user
}

func isAuthenticatedTokenRequest(req *http.Request) bool {
	switch req.URL.Path {
	case "/login/oauth/userinfo":
		fallthrough
	case "/login/oauth/introspect":
		return true
	}
	return false
}
