package utils

import (
	"net/url"
)

func GetQSValue(urlStr string, key string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	values := u.Query()
	return values.Get(key)
}
