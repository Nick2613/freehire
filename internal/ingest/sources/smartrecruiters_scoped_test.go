package sources

import (
	"context"
	"testing"
)

func TestSmartRecruitersScopedBoardKeepsSearchOutOfDetailURL(t *testing.T) {
	f := (&routedHTTP{}).
		route("/RedBull/postings/1", `{"postingUrl":"https://jobs.example/1","jobAd":{"sections":{"jobDescription":{"text":"<p>Build pipelines.</p>"}}}}`).
		route("/RedBull/postings?limit=100&offset=0&q=VCARB", `{"totalFound":2,"content":[{"id":"1","name":"VCARB Data Engineer"}]}`).
		route("/RedBull/postings?limit=100&offset=1&q=VCARB", `{"totalFound":2,"content":[{"id":"2","name":"VCARB Software Engineer"}]}`).
		route("/RedBull/postings/2", `{"postingUrl":"https://jobs.example/2","jobAd":{"sections":{"jobDescription":{"text":"<p>Build software.</p>"}}}}`)
	jobs, err := NewSmartRecruiters(f).Fetch(context.Background(), CompanyEntry{Company: "Racing Bulls", Board: "RedBull?q=VCARB"})
	if err != nil || len(jobs) != 2 {
		t.Fatalf("Fetch = %+v, %v", jobs, err)
	}
	for _, j := range jobs {
		if j.Company != "Racing Bulls" || j.Unreadable || j.Description == "" {
			t.Fatalf("scoped vacancy did not hydrate: %+v", j)
		}
	}
}

func TestSmartRecruitersScopedBoardNeverSilentlyWidens(t *testing.T) {
	for _, board := range []string{"RedBull?q=", "RedBull?typo=VCARB", "RedBull?q=one&q=two", "RedBull?q=%zz", "RedBull?q=VCARB&other=x", "https://example.com"} {
		if _, _, err := smartRecruitersBoard(board); err == nil {
			t.Errorf("accepted malformed scope %q", board)
		}
	}
	if company, keyword, err := smartRecruitersBoard("WilliamsRacing"); err != nil || company != "WilliamsRacing" || keyword != "" {
		t.Fatalf("ordinary tenant changed: %q, %q, %v", company, keyword, err)
	}
}
