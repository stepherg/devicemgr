package translate

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/xmidt-org/talaria/devicemgr"
)

var (
	errEmptyNames    = errors.New("wdmp: empty names")
	errMissingTable  = errors.New("wdmp: missing table")
	errMissingRowRef = errors.New("wdmp: missing row reference")
)

// BuildGet constructs a WDMP GET or GET_ATTRIBUTES payload.
func BuildGet(names []string, includeAttrs bool, attrs string) ([]byte, error) {
	if len(names) == 0 {
		return nil, errEmptyNames
	}
	payload := map[string]interface{}{
		"names":   names,
		"command": "GET",
	}
	if includeAttrs {
		payload["command"] = "GET_ATTRIBUTES"
		if attrs != "" {
			payload["attributes"] = attrs
		}
	}
	return json.Marshal(payload)
}

// BuildSet determines SET vs SET_ATTRS based on parameters.
func BuildSet(params []devicemgr.SetParameter, cas *devicemgr.CASCondition) ([]byte, error) {
	cmd := "SET"
	allAttr := true
	arr := make([]map[string]interface{}, 0, len(params))
	for _, p := range params {
		m := map[string]interface{}{}
		if p.Name == "" {
			return nil, devicemgr.ErrInvalidParameter
		}
		m["name"] = p.Name
		if p.Attributes != nil && p.Value == nil {
			m["attributes"] = p.Attributes
		} else {
			allAttr = false
			if p.Value != nil {
				m["value"] = p.Value
			}
			if p.TypeHint != "" {
				m["dataType"] = p.TypeHint
			}
			if p.Attributes != nil {
				m["attributes"] = p.Attributes
			}
		}
		arr = append(arr, m)
	}
	if allAttr {
		cmd = "SET_ATTRIBUTES"
	}
	root := map[string]interface{}{"command": cmd, "parameters": arr}
	if cas != nil {
		root["newCid"] = cas.NewCID
		if cas.OldCID != "" {
			root["oldCid"] = cas.OldCID
		}
	}
	return json.Marshal(root)
}

func BuildAddRow(table string, row map[string]interface{}) ([]byte, error) {
	if strings.TrimSpace(table) == "" {
		return nil, errMissingTable
	}
	if len(row) == 0 {
		return nil, devicemgr.ErrInvalidParameter
	}
	return json.Marshal(map[string]interface{}{"command": "ADD_ROW", "table": table, "row": row})
}

func BuildReplaceRows(table string, rows []map[string]interface{}) ([]byte, error) {
	if strings.TrimSpace(table) == "" {
		return nil, errMissingTable
	}
	if len(rows) == 0 {
		return nil, devicemgr.ErrInvalidParameter
	}
	return json.Marshal(map[string]interface{}{"command": "REPLACE_ROWS", "table": table, "rows": rows})
}

func BuildDeleteRow(rowRef string) ([]byte, error) {
	if strings.TrimSpace(rowRef) == "" {
		return nil, errMissingRowRef
	}
	return json.Marshal(map[string]interface{}{"command": "DELETE_ROW", "row": rowRef})
}
