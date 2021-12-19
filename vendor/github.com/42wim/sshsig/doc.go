/*
Package sshsig implements signing/verifying armored SSH signatures.
You can use this package to sign data and verify signatures using your ssh private keys or your ssh agent.
It gives the same output as using `ssh-keygen`, eg when signing `ssh-keygen -Y sign -f keyfile -n namespace data`

This code is based upon work by https://github.com/sigstore/rekor

References:
	- https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig
*/
package sshsig
