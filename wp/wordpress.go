package wp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type (
	nameExtract struct {
		Name string `json:"name"`
	}

	WpInfo struct {
		Name       string
		URL        string
		Tags       []string
		Categories []string
	}
)

func Check(record string) (WpInfo, error) {
	url := fmt.Sprintf("https://%s/wp-json", record)
	resp, err := get(url)
	if err != nil {
		return WpInfo{}, err
	}
	name := nameExtract{}
	ct, _ := resp.Header["Content-Type"]
	if resp.StatusCode == 200 && ct != nil && strings.HasPrefix(ct[0], "application/json") {
		err = json.NewDecoder(resp.Body).Decode(&name)
		if err != nil {
			return WpInfo{}, err
		}

		ret := WpInfo{
			name.Name,
			fmt.Sprintf("https://%s", record),
			analyze(fmt.Sprintf("%s/wp/v2/tags", url)),
			analyze(fmt.Sprintf("%s/wp/v2/categories", url)),
		}
		return ret, nil
	}
	return WpInfo{}, err
}

func analyze(url string) []string {
	names := []nameExtract{}

	resp, err := get(url)
	if err != nil {
		logrus.Error(err)
		return []string{}
	}
	err = json.NewDecoder(resp.Body).Decode(&names)
	if err != nil {
		logrus.Error(err)
		return []string{}
	}

	nameList := []string{}
	for _, t := range names {
		nameList = append(nameList, t.Name)
	}
	return nameList
}

func get(url string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &http.Response{}, err
	}
	req = req.WithContext(ctx)
	return http.DefaultClient.Do(req)
}
