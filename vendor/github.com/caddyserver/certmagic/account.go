// Copyright 2015 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certmagic

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/mholt/acmez/acme"
)

// getAccount either loads or creates a new account, depending on if
// an account can be found in storage for the given CA + email combo.
func (am *ACMEManager) getAccount(ca, email string) (acme.Account, error) {
	acct, err := am.loadAccount(ca, email)
	if err != nil {
		if _, ok := err.(ErrNotExist); ok {
			return am.newAccount(email)
		}
		return acct, err
	}
	return acct, err
}

// loadAccount loads an account from storage, but does not create a new one.
func (am *ACMEManager) loadAccount(ca, email string) (acme.Account, error) {
	regBytes, err := am.config.Storage.Load(am.storageKeyUserReg(ca, email))
	if err != nil {
		return acme.Account{}, err
	}
	keyBytes, err := am.config.Storage.Load(am.storageKeyUserPrivateKey(ca, email))
	if err != nil {
		return acme.Account{}, err
	}

	var acct acme.Account
	err = json.Unmarshal(regBytes, &acct)
	if err != nil {
		return acct, err
	}
	acct.PrivateKey, err = decodePrivateKey(keyBytes)
	if err != nil {
		return acct, fmt.Errorf("could not decode account's private key: %v", err)
	}

	return acct, nil
}

// newAccount generates a new private key for a new ACME account, but
// it does not register or save the account.
func (*ACMEManager) newAccount(email string) (acme.Account, error) {
	var acct acme.Account
	if email != "" {
		acct.Contact = []string{"mailto:" + email} // TODO: should we abstract the contact scheme?
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return acct, fmt.Errorf("generating private key: %v", err)
	}
	acct.PrivateKey = privateKey
	return acct, nil
}

// GetAccount first tries loading the account with the associated private key from storage.
// If it does not exist in storage, it will be retrieved from the ACME server and added to storage.
// The account must already exist; it does not create a new account.
func (am *ACMEManager) GetAccount(ctx context.Context, privateKeyPEM []byte) (acme.Account, error) {
	account, err := am.loadAccountByKey(ctx, privateKeyPEM)
	if err != nil {
		if _, ok := err.(ErrNotExist); ok {
			account, err = am.lookUpAccount(ctx, privateKeyPEM)
		} else {
			return account, err
		}
	}
	return account, err
}

// loadAccountByKey loads the account with the given private key from storage, if it exists.
// If it does not exist, an error of type ErrNotExist is returned. This is not very efficient
// for lots of accounts.
func (am *ACMEManager) loadAccountByKey(ctx context.Context, privateKeyPEM []byte) (acme.Account, error) {
	accountList, err := am.config.Storage.List(am.storageKeyUsersPrefix(am.CA), false)
	if err != nil {
		return acme.Account{}, err
	}
	for _, accountFolderKey := range accountList {
		email := path.Base(accountFolderKey)
		keyBytes, err := am.config.Storage.Load(am.storageKeyUserPrivateKey(am.CA, email))
		if err != nil {
			return acme.Account{}, err
		}
		if bytes.Equal(bytes.TrimSpace(keyBytes), bytes.TrimSpace(privateKeyPEM)) {
			return am.loadAccount(am.CA, email)
		}
	}
	return acme.Account{}, ErrNotExist(fmt.Errorf("no account found with that key"))
}

// lookUpAccount looks up the account associated with privateKeyPEM from the ACME server.
// If the account is found by the server, it will be saved to storage and returned.
func (am *ACMEManager) lookUpAccount(ctx context.Context, privateKeyPEM []byte) (acme.Account, error) {
	client, err := am.newACMEClient(false)
	if err != nil {
		return acme.Account{}, fmt.Errorf("creating ACME client: %v", err)
	}

	privateKey, err := decodePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return acme.Account{}, fmt.Errorf("decoding private key: %v", err)
	}

	// look up the account
	account := acme.Account{PrivateKey: privateKey}
	account, err = client.GetAccount(ctx, account)
	if err != nil {
		return acme.Account{}, fmt.Errorf("looking up account with server: %v", err)
	}

	// save the account details to storage
	err = am.saveAccount(client.Directory, account)
	if err != nil {
		return account, fmt.Errorf("could not save account to storage: %v", err)
	}

	return account, nil
}

// saveAccount persists an ACME account's info and private key to storage.
// It does NOT register the account via ACME or prompt the user.
func (am *ACMEManager) saveAccount(ca string, account acme.Account) error {
	regBytes, err := json.MarshalIndent(account, "", "\t")
	if err != nil {
		return err
	}
	keyBytes, err := encodePrivateKey(account.PrivateKey)
	if err != nil {
		return err
	}
	// extract primary contact (email), without scheme (e.g. "mailto:")
	primaryContact := getPrimaryContact(account)
	all := []keyValue{
		{
			key:   am.storageKeyUserReg(ca, primaryContact),
			value: regBytes,
		},
		{
			key:   am.storageKeyUserPrivateKey(ca, primaryContact),
			value: keyBytes,
		},
	}
	return storeTx(am.config.Storage, all)
}

// getEmail does everything it can to obtain an email address
// from the user within the scope of memory and storage to use
// for ACME TLS. If it cannot get an email address, it does nothing
// (If user is prompted, it will warn the user of
// the consequences of an empty email.) This function MAY prompt
// the user for input. If allowPrompts is false, the user
// will NOT be prompted and an empty email may be returned.
func (am *ACMEManager) getEmail(allowPrompts bool) error {
	leEmail := am.Email

	// First try package default email, or a discovered email address
	if leEmail == "" {
		leEmail = DefaultACME.Email
	}
	if leEmail == "" {
		discoveredEmailMu.Lock()
		leEmail = discoveredEmail
		discoveredEmailMu.Unlock()
	}

	// Then try to get most recent user email from storage
	var gotRecentEmail bool
	if leEmail == "" {
		leEmail, gotRecentEmail = am.mostRecentAccountEmail(am.CA)
	}
	if !gotRecentEmail && leEmail == "" && allowPrompts {
		// Looks like there is no email address readily available,
		// so we will have to ask the user if we can.
		var err error
		leEmail, err = am.promptUserForEmail()
		if err != nil {
			return err
		}

		// User might have just signified their agreement
		am.Agreed = DefaultACME.Agreed
	}

	// save the email for later and ensure it is consistent
	// for repeated use; then update cfg with the email
	leEmail = strings.TrimSpace(strings.ToLower(leEmail))
	discoveredEmailMu.Lock()
	if discoveredEmail == "" {
		discoveredEmail = leEmail
	}
	discoveredEmailMu.Unlock()
	am.Email = leEmail

	return nil
}

// promptUserForEmail prompts the user for an email address
// and returns the email address they entered (which could
// be the empty string). If no error is returned, then Agreed
// will also be set to true, since continuing through the
// prompt signifies agreement.
func (am *ACMEManager) promptUserForEmail() (string, error) {
	// prompt the user for an email address and terms agreement
	reader := bufio.NewReader(stdin)
	am.promptUserAgreement("")
	fmt.Println("Please enter your email address to signify agreement and to be notified")
	fmt.Println("in case of issues. You can leave it blank, but we don't recommend it.")
	fmt.Print("  Email address: ")
	leEmail, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("reading email address: %v", err)
	}
	leEmail = strings.TrimSpace(leEmail)
	DefaultACME.Agreed = true
	return leEmail, nil
}

// promptUserAgreement simply outputs the standard user
// agreement prompt with the given agreement URL.
// It outputs a newline after the message.
func (am *ACMEManager) promptUserAgreement(agreementURL string) {
	userAgreementPrompt := `Your sites will be served over HTTPS automatically using an automated CA.
By continuing, you agree to the CA's terms of service`
	if agreementURL == "" {
		fmt.Printf("\n\n%s.\n", userAgreementPrompt)
		return
	}
	fmt.Printf("\n\n%s at:\n  %s\n", userAgreementPrompt, agreementURL)
}

// askUserAgreement prompts the user to agree to the agreement
// at the given agreement URL via stdin. It returns whether the
// user agreed or not.
func (am *ACMEManager) askUserAgreement(agreementURL string) bool {
	am.promptUserAgreement(agreementURL)
	fmt.Print("Do you agree to the terms? (y/n): ")

	reader := bufio.NewReader(stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))

	return answer == "y" || answer == "yes"
}

func storageKeyACMECAPrefix(issuerKey string) string {
	return path.Join(prefixACME, StorageKeys.Safe(issuerKey))
}

func (am *ACMEManager) storageKeyCAPrefix(caURL string) string {
	return storageKeyACMECAPrefix(am.issuerKey(caURL))
}

func (am *ACMEManager) storageKeyUsersPrefix(caURL string) string {
	return path.Join(am.storageKeyCAPrefix(caURL), "users")
}

func (am *ACMEManager) storageKeyUserPrefix(caURL, email string) string {
	if email == "" {
		email = emptyEmail
	}
	return path.Join(am.storageKeyUsersPrefix(caURL), StorageKeys.Safe(email))
}

func (am *ACMEManager) storageKeyUserReg(caURL, email string) string {
	return am.storageSafeUserKey(caURL, email, "registration", ".json")
}

func (am *ACMEManager) storageKeyUserPrivateKey(caURL, email string) string {
	return am.storageSafeUserKey(caURL, email, "private", ".key")
}

// storageSafeUserKey returns a key for the given email, with the default
// filename, and the filename ending in the given extension.
func (am *ACMEManager) storageSafeUserKey(ca, email, defaultFilename, extension string) string {
	if email == "" {
		email = emptyEmail
	}
	email = strings.ToLower(email)
	filename := am.emailUsername(email)
	if filename == "" {
		filename = defaultFilename
	}
	filename = StorageKeys.Safe(filename)
	return path.Join(am.storageKeyUserPrefix(ca, email), filename+extension)
}

// emailUsername returns the username portion of an email address (part before
// '@') or the original input if it can't find the "@" symbol.
func (*ACMEManager) emailUsername(email string) string {
	at := strings.Index(email, "@")
	if at == -1 {
		return email
	} else if at == 0 {
		return email[1:]
	}
	return email[:at]
}

// mostRecentAccountEmail finds the most recently-written account file
// in storage. Since this is part of a complex sequence to get a user
// account, errors here are discarded to simplify code flow in
// the caller, and errors are not important here anyway.
func (am *ACMEManager) mostRecentAccountEmail(caURL string) (string, bool) {
	accountList, err := am.config.Storage.List(am.storageKeyUsersPrefix(caURL), false)
	if err != nil || len(accountList) == 0 {
		return "", false
	}

	// get all the key infos ahead of sorting, because
	// we might filter some out
	stats := make(map[string]KeyInfo)
	for i := 0; i < len(accountList); i++ {
		u := accountList[i]
		keyInfo, err := am.config.Storage.Stat(u)
		if err != nil {
			continue
		}
		if keyInfo.IsTerminal {
			// I found a bug when macOS created a .DS_Store file in
			// the users folder, and CertMagic tried to use that as
			// the user email because it was newer than the other one
			// which existed... sure, this isn't a perfect fix but
			// frankly one's OS shouldn't mess with the data folder
			// in the first place.
			accountList = append(accountList[:i], accountList[i+1:]...)
			i--
			continue
		}
		stats[u] = keyInfo
	}

	sort.Slice(accountList, func(i, j int) bool {
		iInfo := stats[accountList[i]]
		jInfo := stats[accountList[j]]
		return jInfo.Modified.Before(iInfo.Modified)
	})

	if len(accountList) == 0 {
		return "", false
	}

	account, err := am.getAccount(caURL, path.Base(accountList[0]))
	if err != nil {
		return "", false
	}

	return getPrimaryContact(account), true
}

// getPrimaryContact returns the first contact on the account (if any)
// without the scheme. (I guess we assume an email address.)
func getPrimaryContact(account acme.Account) string {
	// TODO: should this be abstracted with some lower-level helper?
	var primaryContact string
	if len(account.Contact) > 0 {
		primaryContact = account.Contact[0]
		if idx := strings.Index(primaryContact, ":"); idx >= 0 {
			primaryContact = primaryContact[idx+1:]
		}
	}
	return primaryContact
}

// When an email address is not explicitly specified, we can remember
// the last one we discovered to avoid having to ask again later.
// (We used to store this in DefaultACME.Email but it was racey; see #127)
var (
	discoveredEmail   string
	discoveredEmailMu sync.Mutex
)

// stdin is used to read the user's input if prompted;
// this is changed by tests during tests.
var stdin = io.ReadWriter(os.Stdin)

// The name of the folder for accounts where the email
// address was not provided; default 'username' if you will,
// but only for local/storage use, not with the CA.
const emptyEmail = "default"
