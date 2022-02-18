package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Payload struct {
	Form map[string]struct {
		Label    string      `json:"label"`
		Type     string      `json:"type"`
		Required bool        `json:"required"`
		Default  interface{} `json:"default"`
		Value    interface{} `json:"value"`
	} `json:"form"`
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		panic(err)
	}

	username, ok := p.Form["username"]
	if !ok {
		panic("username not found in form data")
	}
	checkType(username.Type, username.Value)
	if username.Value.(string) != "jolheiser" {
		panic("username should be jolheiser")
	}

	favNum, ok := p.Form["favorite-number"]
	if !ok {
		panic("favorite-number not found in form data")
	}
	checkType(favNum.Type, favNum.Value)
	if username.Value.(float64) != 12 {
		panic("favNum should be 12")
	}

	pineapple, ok := p.Form["pineapple"]
	if !ok {
		panic("pineapple not found in form data")
	}
	checkType(pineapple.Type, pineapple.Value)
	if pineapple.Value.(bool) {
		panic("pineapple should be false")
	}

	secret, ok := p.Form["secret"]
	if !ok {
		panic("secret not found in form data")
	}
	checkType(secret.Type, secret.Value)
	if secret.Value.(string) != "sn34ky" {
		panic("secret should be sn34ky")
	}
}

func checkType(typ string, val interface{}) {
	var ok bool
	switch typ {
	case "text", "secret":
		_, ok = val.(string)
	case "bool":
		_, ok = val.(bool)
	case "number":
		_, ok = val.(float64)
	}
	if !ok {
		panic(fmt.Sprintf("unexpected type %q for %v", typ, val))
	}
}

// override panic
func panic(v interface{}) {
	fmt.Println(v)
	os.Exit(1)
}
