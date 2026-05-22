package parsers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func GetHtml(url string) (result string, error error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.27 Safari/537.36`)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET error: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET status error: %v", resp.StatusCode)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %v", err)
	}

	return string(bytes), nil
}

func parsePrice(data string) (float32, error) {
	price, err := strconv.ParseFloat(data, 32)
	if err != nil {
		return 0, fmt.Errorf("parsing float: %v %s", err, data)
	}

	return float32(price), nil
}

