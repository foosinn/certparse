package title

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var r = regexp.MustCompile("(?i)<title>([^<]+)")

func GetInfo(url string, atoms []atom.Atom) (ti TitleInfo, err error) {
	var curAtom atom.Atom
	ti = TitleInfo{
		Url:     url,
		TagVals: map[string][]string{},
		Err:     nil,
	}
	r, err := download(url)
	if err != nil {
		return
	}

	z := html.NewTokenizer(r)
TOKENIZER:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				break TOKENIZER
			} else {
				ti.Err = z.Err()
				return
			}
		case html.StartTagToken:
			t := z.Token()
			curAtom = t.DataAtom
			key, values := parseTag(&t)
			if len(values) > 0 {
				ti.TagVals[key] = values
			}
		case html.EndTagToken:
			t := z.Token()
			if t.DataAtom == atom.Head {
				return
			}
		case html.TextToken:
			t := z.Token()
			for _, wantAtom := range atoms {
				data := strings.TrimSpace(t.Data)
				if curAtom == wantAtom && data != "" {
					ti.TagVals[curAtom.String()] = []string{data}
				}

			}
		}
	}
	return
}

func download(url string) (io.Reader, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s", url), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func parseTag(t *html.Token) (key string, values []string) {
	key = t.Data
	for _, attr := range t.Attr {
		if attr.Key == "name" {
			if badMeta(attr.Val) {
				continue
			}
			key = fmt.Sprintf("%s %s", key, attr.Val)
		}
		if attr.Key == "content" {
			values = strings.Split(attr.Val, ",")
			for index, value := range values {
				values[index] = strings.TrimSpace(value)
			}
		}
	}
	return
}

func badMeta(attrName string) bool {
	bad := []string{
		"viewport",
		"ROBOTS",
		"apple",
	}
	for _, b := range bad {
		if strings.HasPrefix(attrName, b) {
			return true
		}
	}
	return false
}
