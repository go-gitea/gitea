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

	"github.com/mholt/acmez/acme"
)

// getAccount either loads or creates a new account, depending on if
// an account can be found in storage for the given CA + email combo.
func (am *ACMEManager) getAccount(ca, email string) (acme.Account, error) {
	regBytes, err := am.config.Storage.Load(am.storageKeyUserReg(ca, email))
	if err != nil {
		if _, ok := err.(ErrNotExist); ok {
			return am.newAccount(email)
		}
		return acme.Account{}, err
	}
	keyBytes, err := am.config.Storage.Load(am.storageKeyUserPrivateKey(ca, email))
	if err != nil {
		if _, ok := err.(ErrNotExist); ok {
			return am.newAccount(email)
		}
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

	// TODO: July 2020 - transition to new ACME lib and account structure;
	// for a while, we will need to convert old accounts to new structure
	acct, err = am.transitionAccountToACMEzJuly2020Format(ca, acct, regBytes)
	if err != nil {
		return acct, fmt.Errorf("one-time account transition: %v", err)
	}

	return acct, err
}

// TODO: this is a temporary transition helper starting July 2020.
// It can go away when we think enough time has passed that most active assets have transitioned.
func (am *ACMEManager) transitionAccountToACMEzJuly2020Format(ca string, acct acme.Account, regBytes []byte) (acme.Account, error) {
	if acct.Status != "" && acct.Location != "" {
		return acct, nil
	}

	var oldAcct struct {
		Email        string `json:"Email"`
		Registration struct {
			Body struct {
				Status                 string          `json:"status"`
				TermsOfServiceAgreed   bool            `json:"termsOfServiceAgreed"`
				Orders                 string          `json:"orders"`
				ExternalAccountBinding json.RawMessage `json:"externalAccountBinding"`
			} `json:"body"`
			URI string `json:"uri"`
		} `json:"Registration"`
	}
	err := json.Unmarshal(regBytes, &oldAcct)
	if err != nil {
		return acct, fmt.Errorf("decoding into old account type: %v", err)
	}

	acct.Status = oldAcct.Registration.Body.Status
	acct.TermsOfServiceAgreed = oldAcct.Registration.Body.TermsOfServiceAgreed
	acct.Location = oldAcct.Registration.URI
	acct.ExternalAccountBinding = oldAcct.Registration.Body.ExternalAccountBinding
	acct.Orders = oldAcct.Registration.Body.Orders
	if oldAcct.Email != "" {
		acct.Contact = []string{"mailto:" + oldAcct.Email}
	}

	err = am.saveAccount(ca, acct)
	if err != nil {
		return acct, fmt.Errorf("saving converted account: %v", err)
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

	// First try package default email
	if leEmail == "" {
		leEmail = DefaultACME.Email // TODO: racey with line 122 (or whichever line assigns to DefaultACME.Email below)
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
	DefaultACME.Email = strings.TrimSpace(strings.ToLower(leEmail)) // TODO: this is racey with line 99
	am.Email = DefaultACME.Email

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

func (am *ACMEManager) storageKeyCAPrefix(caURL string) string {
	return path.Join(prefixACME, StorageKeys.Safe(am.issuerKey(caURL)))
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
	for i, u := range accountList {
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

// agreementTestURL is set during tests to skip requiring
// setting up an entire ACME CA endpoint.
var agreementTestURL string

// stdin is used to read the user's input if prompted;
// this is changed by tests during tests.
var stdin = io.ReadWriter(os.Stdin)

// The name of the folder for accounts where the email
// address was not provided; default 'username' if you will,
// but only for local/storage use, not with the CA.
const emptyEmail = "default"
