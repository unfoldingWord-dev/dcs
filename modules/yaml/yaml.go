// Copyright 2019 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*** DCS Customizations - Module for YAML rendering ***/

package yaml

import (
	"fmt"

	"github.com/microcosm-cc/bluemonday"
	"gopkg.in/yaml.v2"
)

var sanitizer = bluemonday.UGCPolicy()

func renderHorizontalHTMLTable(m yaml.MapSlice) (string, error) {
	var err error
	var thead, tbody, table string
	var mi yaml.MapItem
	for _, mi = range m {
		key := mi.Key
		value := mi.Value

		switch slice := key.(type) {
		case yaml.MapSlice:
			key, err = renderHorizontalHTMLTable(slice)
			if err != nil {
				return "", err
			}
		}
		thead += fmt.Sprintf("<th>%v</th>", key)

		switch switchedValue := value.(type) {
		case yaml.MapSlice:
			value, err = renderHorizontalHTMLTable(switchedValue)
			if err != nil {
				return "", err
			}
		case []interface{}:
			v := make([]yaml.MapSlice, len(switchedValue))
			for i, vs := range switchedValue {
				switch vs := vs.(type) {
				case yaml.MapSlice:
					v[i] = vs
				default:
					return "", fmt.Errorf("Unexpected type %T, expected MapSlice", vs)
				}
			}
			value, err = renderVerticalHTMLTable(v)
			if err != nil {
				return "", err
			}
		}
		tbody += fmt.Sprintf("<td>%v</td>", value)
	}

	table = ""
	if len(thead) > 0 {
		table = fmt.Sprintf(`<table data="yaml-metadata"><thead><tr>%s</tr></thead><tbody><tr>%s</tr></table>`, thead, tbody)
	}
	return table, nil
}

func renderVerticalHTMLTable(m []yaml.MapSlice) (string, error) {
	var err error
	var ms yaml.MapSlice
	var mi yaml.MapItem
	var table string

	for _, ms = range m {
		table += `<table data="yaml-metadata">`
		for _, mi = range ms {
			key := mi.Key
			value := mi.Value

			table += `<tr>`
			switch switchedKey := key.(type) {
			case yaml.MapSlice:
				key, err = renderHorizontalHTMLTable(switchedKey)
				if err != nil {
					return "", err
				}
			case []interface{}:
				var ks string
				for _, ki := range switchedKey {
					switch ki := ki.(type) {
					case yaml.MapSlice:
						horiz, err := renderHorizontalHTMLTable(ki)
						if err != nil {
							return "", err
						}
						ks += horiz
					default:
						return "", fmt.Errorf("Unexpected type %T, expected MapSlice", ki)
					}
				}
				key = ks
			}
			table += fmt.Sprintf("<td>%v</td>", key)

			switch switchedValue := value.(type) {
			case yaml.MapSlice:
				value, err = renderHorizontalHTMLTable(switchedValue)
				if err != nil {
					return "", err
				}
			case []interface{}:
				v := make([]yaml.MapSlice, len(switchedValue))
				for i, vs := range switchedValue {
					switch vs := vs.(type) {
					case yaml.MapSlice:
						v[i] = vs
					default:
						return "", fmt.Errorf("Unexpected type %T, expected MapSlice", vs)
					}
				}
				value, err = renderVerticalHTMLTable(v)
				if err != nil {
					return "", err
				}
			}

			switch key {
			case "slug":
				value = fmt.Sprintf(`<a href="content/%v.md">%v</a>`, value, value)
			case "link":
				value = fmt.Sprintf(`<a href="%v/01.md">%v</a>`, value, value)
			}
			table += fmt.Sprintf("<td>%v</td>", value)
			table += `</tr>`
		}
		table += "</table>"
	}

	return table, nil
}

// Render render yaml contents as html
func Render(data []byte) ([]byte, error) {
	mss := []yaml.MapSlice{}

	if len(data) < 1 {
		return data, nil
	}

	if err := yaml.Unmarshal(data, &mss); err != nil {
		ms := yaml.MapSlice{}
		if err := yaml.Unmarshal(data, &ms); err != nil {
			return nil, err
		}
		table, err := renderHorizontalHTMLTable(ms)
		return []byte(table), err
	}
	table, err := renderVerticalHTMLTable(mss)
	return []byte(table), err
}

// RenderSanitized render yaml as sanitized html
func RenderSanitized(rawBytes []byte) ([]byte, error) {
	result, err := Render(rawBytes)
	if err != nil {
		return nil, err
	}
	return sanitizer.SanitizeBytes(result), nil
}
