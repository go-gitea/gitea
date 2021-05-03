// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import "github.com/go-swagger/go-swagger/cmd/swagger/commands/generate"

// Generate command to group all generator commands together
type Generate struct {
	Model     *generate.Model     `command:"model"`
	Operation *generate.Operation `command:"operation"`
	Support   *generate.Support   `command:"support"`
	Server    *generate.Server    `command:"server"`
	Spec      *generate.SpecFile  `command:"spec"`
	Client    *generate.Client    `command:"client"`
	Cli       *generate.Cli       `command:"cli"`
	Markdown  *generate.Markdown  `command:"markdown"`
}
