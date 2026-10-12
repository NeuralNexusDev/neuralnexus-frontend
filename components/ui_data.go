package components

import "strings"

func fieldDescription(id string, help string, errText string) string {
	var ids []string
	if errText != "" {
		ids = append(ids, id+"-error")
	}
	if help != "" {
		ids = append(ids, id+"-help")
	}
	return strings.Join(ids, " ")
}
