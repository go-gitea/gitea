// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional

import (
	"code.gitea.io/gitea/modules/json"

	"gopkg.in/yaml.v3"
)

func (o *Option[T]) UnmarshalJSON(data []byte) error {
	var v *T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*o = FromPtr(v)
	return nil
}

func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.Has() {
		return []byte("null"), nil
	}

	return json.Marshal(o.Value())
}

func (o *Option[T]) UnmarshalYAML(value *yaml.Node) error {
	var v *T
	if err := value.Decode(&v); err != nil {
		return err
	}
	*o = FromPtr(v)
	return nil
}

func (o Option[T]) MarshalYAML() (interface{}, error) {
	if !o.Has() {
		return nil, nil
	}

	value := new(yaml.Node)
	err := value.Encode(o.Value())
	return value, err
}
