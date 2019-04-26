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
package flbconfig

import "errors"

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Section struct {
	Name      string     `json:"name"`
	KeyValues []KeyValue `json:"keyValuePairs"`
}

type File struct {
	Name     string    `json:"name"`
	Sections []Section `json:"sections"`
}

func Parse(name, input string) (File, error) {
	f := File{
		Name: name,
	}
	l := NewLexer(input)
	l.Run()

	var (
		requireNewline    bool
		processingSection bool
		section           Section

		processingKV bool
		key          string
	)

	for _, t := range l.Tokens {
		if requireNewline {
			if t.Type != TokenNewLine {
				return File{}, errors.New("newline required")
			}
			requireNewline = false
			continue
		}

		switch t.Type {
		case TokenError:
			return File{}, errors.New(t.Value)
		case TokenEOF:
			if processingKV || processingSection {
				return File{}, errors.New("unexpected EOF")
			}
			f.Sections = append(f.Sections, section)
			return f, nil
		case TokenNewLine:
			if processingKV || processingSection {
				return File{}, errors.New("unexpected newline")
			}
		case TokenLeftBracket:
			if processingKV || processingSection {
				return File{}, errors.New("unexpected left bracket")
			}
			f.Sections = append(f.Sections, section)
			processingSection = true
			section = Section{}
		case TokenSection:
			if !processingSection {
				return File{}, errors.New("unexpected section")
			}
			section.Name = t.Value
		case TokenRightBracket:
			if !processingSection {
				return File{}, errors.New("unexpected right bracket")
			}
			processingSection = false
			requireNewline = true
		case TokenKey:
			if processingKV || processingSection {
				return File{}, errors.New("unexpected key")
			}
			processingKV = true
			key = t.Value
		case TokenValue:
			if !processingKV {
				return File{}, errors.New("unexpected value")
			}
			section.KeyValues = append(section.KeyValues, KeyValue{
				Key:   key,
				Value: t.Value,
			})
			processingKV = false
			key = ""
		}
	}

	return f, nil
}
