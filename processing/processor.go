package processing

import (
	"fmt"
	"leprechaun/parsers"
	"leprechaun/submitting"
)

func ParseSilpoLink(url string) (result string, err error) {
	body, err := parsers.GetHtml(url)
	if err != nil {
		return "", fmt.Errorf("HTTP error: %v", err)
	}

	items, t, err := parsers.ParseSilpoChequeHtml(body)

	if err != nil {
		return "", fmt.Errorf("parsing error: %v", err)
	}

	res, err := submitting.SubmitGoogleForm(items, t)

	if err != nil {
		return "", fmt.Errorf("google form error: %v", err)
	}

	return res, nil
}
