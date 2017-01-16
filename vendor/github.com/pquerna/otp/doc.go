/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Package otp implements both HOTP and TOTP based
// one time passcodes in a Google Authenticator compatible manner.
//
// When adding a TOTP for a user, you must store the "secret" value
// persistently. It is recommend to store the secret in an encrypted field in your
// datastore.  Due to how TOTP works, it is not possible to store a hash
// for the secret value like you would a password.
//
// To enroll a user, you must first generate an OTP for them.  Google
// Authenticator supports using a QR code as an enrollment method:
//
//	import (
//		"github.com/pquerna/otp/totp"
//
//		"bytes"
//		"image/png"
//	)
//
//	key, err := totp.Generate(totp.GenerateOpts{
//			Issuer: "Example.com",
//			AccountName: "alice@example.com",
//	})
//
//	// Convert TOTP key into a QR code encoded as a PNG image.
//	var buf bytes.Buffer
//	img, err := key.Image(200, 200)
//	png.Encode(&buf, img)
//
//	// display the QR code to the user.
//	display(buf.Bytes())
//
//	// Now Validate that the user's successfully added the passcode.
//	passcode := promptForPasscode()
//	valid := totp.Validate(passcode, key.Secret())
//
//	if valid {
//		// User successfully used their TOTP, save it to your backend!
//		storeSecret("alice@example.com", key.Secret())
//	}
//
// Validating a TOTP passcode is very easy, just prompt the user for a passcode
// and retrieve the associated user's previously stored secret.
//	import "github.com/pquerna/otp/totp"
//
//	passcode := promptForPasscode()
//	secret := getSecret("alice@example.com")
//
//	valid := totp.Validate(passcode, secret)
//
//	if valid {
//		// Success! continue login process.
//	}
package otp
