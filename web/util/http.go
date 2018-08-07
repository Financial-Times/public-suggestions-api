package util

import (
	"fmt"
	"net/http"
)

func GetSingleValueQueryParameter(req *http.Request, param string, allowed ...string) (string, bool, error) {
	values, found, err := GetMultipleValueQueryParameter(req, param, allowed...)
	if err != nil {
		return "", found, err
	}
	if len(values) > 1 {
		return "", found, fmt.Errorf("specified multiple %v query parameters in the URL", param)
	}
	if len(values) < 1 {
		return "", found, nil
	}

	return values[0], found, nil
}

func GetMultipleValueQueryParameter(req *http.Request, param string, allowed ...string) ([]string, bool, error) {
	query := req.URL.Query()
	values, found := query[param]
	for _, value := range values {
		allow := false
		for _, a := range allowed {
			if value == a {
				allow = true
			}
		}
		if !allow {
			return []string{}, found, fmt.Errorf("'%s' is not a valid value for parameter '%s'", value, param)
		}
	}
	return values, found, nil
}
