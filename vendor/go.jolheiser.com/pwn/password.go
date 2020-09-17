package pwn

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

const passwordURL = "https://api.pwnedpasswords.com/range/"

// CheckPassword returns the number of times a password has been compromised
// Adding padding will make requests more secure, however is also slower
// because artificial responses will be added to the response
// For more information, see https://www.troyhunt.com/enhancing-pwned-passwords-privacy-with-padding/
func (c *Client) CheckPassword(pw string, padding bool) (int, error) {
	if strings.TrimSpace(pw) == "" {
		return -1, ErrEmptyPassword{}
	}

	sha := sha1.New()
	sha.Write([]byte(pw))
	enc := hex.EncodeToString(sha.Sum(nil))
	prefix, suffix := enc[:5], enc[5:]

	req, err := newRequest(c.ctx, http.MethodGet, fmt.Sprintf("%s%s", passwordURL, prefix), nil)
	if err != nil {
		return -1, nil
	}
	if padding {
		req.Header.Add("Add-Padding", "true")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return -1, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	for _, pair := range strings.Split(string(body), "\n") {
		parts := strings.Split(pair, ":")
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(suffix, parts[0]) {
			count, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err != nil {
				return -1, err
			}
			return int(count), nil
		}
	}
	return 0, nil
}
