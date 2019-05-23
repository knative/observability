/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package flbconfig_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/knative/observability/pkg/sink/flbconfig"
)

func TestLex(t *testing.T) {
	testCases := map[string]struct {
		input          string
		expectedTokens []flbconfig.Token
	}{
		"empty": {
			input: "",
			expectedTokens: []flbconfig.Token{
				{
					Type: flbconfig.TokenEOF,
				},
			},
		},
		"json value": {
			input: `
[section]
key [{"some": "json"}]
`,
			expectedTokens: []flbconfig.Token{
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenLeftBracket,
					Value: "[",
				},
				{
					Type:  flbconfig.TokenSection,
					Value: "section",
				},
				{
					Type:  flbconfig.TokenRightBracket,
					Value: "]",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenKey,
					Value: "key",
				},
				{
					Type:  flbconfig.TokenValue,
					Value: `[{"some": "json"}]`,
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type: flbconfig.TokenEOF,
				},
			},
		},
		"extra whitespace": {
			input: `
				[section]
				key  val
			`,
			expectedTokens: []flbconfig.Token{
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenLeftBracket,
					Value: "[",
				},
				{
					Type:  flbconfig.TokenSection,
					Value: "section",
				},
				{
					Type:  flbconfig.TokenRightBracket,
					Value: "]",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenKey,
					Value: "key",
				},
				{
					Type:  flbconfig.TokenValue,
					Value: "val",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type: flbconfig.TokenEOF,
				},
			},
		},
		"normal": {
			input: `

[sectionA]
keyA1 valA1
keyA2 valA2

[sectionB]
keyB1 valB1
keyB2 valB2
keyB1.subkey valB1s

`,
			expectedTokens: []flbconfig.Token{
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenLeftBracket,
					Value: "[",
				},
				{
					Type:  flbconfig.TokenSection,
					Value: "sectionA",
				},
				{
					Type:  flbconfig.TokenRightBracket,
					Value: "]",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenKey,
					Value: "keyA1",
				},
				{
					Type:  flbconfig.TokenValue,
					Value: "valA1",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenKey,
					Value: "keyA2",
				},
				{
					Type:  flbconfig.TokenValue,
					Value: "valA2",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenLeftBracket,
					Value: "[",
				},
				{
					Type:  flbconfig.TokenSection,
					Value: "sectionB",
				},
				{
					Type:  flbconfig.TokenRightBracket,
					Value: "]",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenKey,
					Value: "keyB1",
				},
				{
					Type:  flbconfig.TokenValue,
					Value: "valB1",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenKey,
					Value: "keyB2",
				},
				{
					Type:  flbconfig.TokenValue,
					Value: "valB2",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenKey,
					Value: "keyB1.subkey",
				},
				{
					Type:  flbconfig.TokenValue,
					Value: "valB1s",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type:  flbconfig.TokenNewLine,
					Value: "\n",
				},
				{
					Type: flbconfig.TokenEOF,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			l := flbconfig.NewLexer(tc.input)
			l.Run()

			if !cmp.Equal(l.Tokens, tc.expectedTokens) {
				t.Error(cmp.Diff(l.Tokens, tc.expectedTokens))
			}
		})
	}
}
