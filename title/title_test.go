package title

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html/atom"
)

func TestGetInfo(t *testing.T) {
	r := bytes.NewReader([]byte(`
<html>
<head>
<title>Hello</title>
<meta name="keywords" content="example, html, head, meta">
</head>
</html>
`))
	got, err := GetTagVals(r, []atom.Atom{atom.Title, atom.Meta})
	if err != nil {
		t.Error(err)
	}
	want := map[string][]string{
		"title":         []string{"Hello"},
		"meta keywords": []string{"example", " html", " head", " meta"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}
