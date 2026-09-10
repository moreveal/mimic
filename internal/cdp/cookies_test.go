package cdp

import "testing"

// Chrome 152.0.7977.83, disposable context: omitted partitionKey deletes only
// the unpartitioned cookie; an explicit key selects exactly that partition.
func TestCDPCookiesPreservePartitionAndEffectiveAttributes(t *testing.T) {
	server, _ := runningServer(t)
	s := &session{page: server.Page}
	first := map[string]any{"topLevelSite": "https://a.test", "hasCrossSiteAncestor": false}
	other := map[string]any{"topLevelSite": "https://b.test", "hasCrossSiteAncestor": true}
	for i, key := range []map[string]any{nil, first, other} {
		p := map[string]any{"url": "https://a.test/", "name": "id", "value": []string{"shared", "first", "other"}[i], "secure": true, "httpOnly": true, "sameSite": "None"}
		if key != nil {
			p["partitionKey"] = key
		}
		if err := s.setCookie(p); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.pageCookies(); len(got) != 3 {
		t.Fatal(got)
	} else {
		for _, item := range got {
			row := item.(map[string]any)
			if row["secure"] != true || row["httpOnly"] != true || row["sameSite"] != "None" || row["session"] != true || row["domain"] != "a.test" {
				t.Fatal(row)
			}
		}
	}
	if err := s.deleteCookies(map[string]any{"url": "https://a.test/", "name": "id"}); err != nil {
		t.Fatal(err)
	}
	if len(s.pageCookies()) != 2 {
		t.Fatal(s.pageCookies())
	}
	if err := s.deleteCookies(map[string]any{"url": "https://a.test/", "name": "id", "partitionKey": first}); err != nil {
		t.Fatal(err)
	}
	rows := server.Page.Cookies().Snapshots()
	if len(rows) != 1 || rows[0].Cookie.Value != "other" || rows[0].PartitionKey.TopLevelSite != "https://b.test" {
		t.Fatal(rows)
	}
	// Rejected opaque keys must not silently write into the first-party jar.
	if err := s.setCookie(map[string]any{"url": "https://a.test/", "name": "bad", "partitionKey": map[string]any{"topLevelSite": "null", "hasCrossSiteAncestor": true}}); err == nil {
		t.Fatal("opaque key accepted")
	}
	if len(server.Page.Cookies().All()) != 1 {
		t.Fatal("rejected write mutated jar")
	}
}
