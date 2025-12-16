package utils

import (
	"fmt"
	"strings"
)

func JoinLabels(InfoLabels map[string]string) string {
	// 拼接 labels
	var result string
	//fmt.Println("===label:", InfoLabels)
	for k, v := range InfoLabels {
		result += fmt.Sprintf("%s=\"%s\",", k, v)
	}

	result = strings.TrimRight(result, ",")
	//fmt.Println("===label:", result)
	return result
}
