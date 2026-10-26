package sources

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type schemaorgFake struct{ pages map[string]string }

func (f schemaorgFake) GetHTML(_ context.Context, raw string) (*html.Node, error) {
	body, ok := f.pages[raw]
	if !ok {
		return nil, errors.New("unreachable page")
	}
	return html.Parse(strings.NewReader(body))
}

func (f schemaorgFake) GetXML(_ context.Context, raw string, target any) error {
	body, ok := f.pages[raw]
	if !ok {
		return errors.New("unreachable sitemap")
	}
	return xml.Unmarshal([]byte(body), target)
}

const schemaorgDetailFixture = `<script type="application/ld+json">{
"@type":"JobPosting","title":"Data &amp; Platform Engineer",
"description":"<p>Build SQL pipelines.</p><script>alert(1)<\/script>",
"datePosted":"2026-10-01T12:00:00Z","employmentType":"FULL_TIME",
"jobLocationType":"TELECOMMUTE",
"jobLocation":[{"address":{"addressLocality":"Grove","addressCountry":"UK"}}]
}</script>`

func TestSchemaOrgSitemapDeduplicatesAndKeepsOnlyOwnJobPages(t *testing.T) {
	f := schemaorgFake{pages: map[string]string{
		"https://careers.example/jobs.xml": `<urlset>
<url><loc>https://careers.example/job/data-jid-1?campaign=one</loc></url>
<url><loc>https://careers.example/job/data-jid-1?campaign=two</loc></url>
<url><loc>https://unrelated.example/job/other</loc></url>
<url><loc>https://careers.example/about</loc></url>
</urlset>`,
		"https://careers.example/job/data-jid-1": schemaorgDetailFixture,
	}}
	jobs, err := NewSchemaOrg(f).Fetch(context.Background(), CompanyEntry{Company: "F1 Team", Board: "careers.example/jobs.xml"})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("Fetch = %v, %v", jobs, err)
	}
	j := jobs[0]
	if j.Title != "Data & Platform Engineer" || j.Company != "F1 Team" || j.Location != "Grove, UK" || !j.Remote || j.WorkMode != "remote" || j.EmploymentType != "full_time" || j.PostedAt == nil {
		t.Fatalf("incorrect mapping: %+v", j)
	}
	if strings.Contains(j.Description, "<script") || strings.Contains(j.URL, "?") || j.ExternalID == "" {
		t.Fatalf("unsafe description or unstable identity: %+v", j)
	}
}

func TestSchemaOrgHTMLListingFollowsPagesAndAcceptsObjectLocation(t *testing.T) {
	f := schemaorgFake{pages: map[string]string{
		"https://careers.example/careers/vacancies?page=1": `<a href="/careers/vacancies/1">First</a>`,
		"https://careers.example/careers/vacancies?page=2": `<a href="/careers/vacancies/2">Second</a>`,
		"https://careers.example/careers/vacancies?page=3": `<a href="/careers/vacancies/2">Second</a>`,
		"https://careers.example/careers/vacancies/1":      schemaorgDetailFixture,
		"https://careers.example/careers/vacancies/2": strings.Replace(schemaorgDetailFixture,
			`[{"address":{"addressLocality":"Grove","addressCountry":"UK"}}]`,
			`{"address":{"addressLocality":"Brackley","addressCountry":"UK"}}`, 1),
	}}
	f.pages["https://careers.example/careers/vacancies/2"] = strings.Replace(f.pages["https://careers.example/careers/vacancies/2"], `"employmentType":"FULL_TIME"`, `"employmentType":["FULL_TIME"]`, 1)
	jobs, err := NewSchemaOrg(f).Fetch(context.Background(), CompanyEntry{Company: "F1 Team", Board: "careers.example/careers/vacancies"})
	if err != nil || len(jobs) != 2 {
		t.Fatalf("Fetch = %v, %v", jobs, err)
	}
	if jobs[0].ExternalID == jobs[1].ExternalID {
		t.Fatal("distinct postings share an identity")
	}
	locations := map[string]bool{}
	for _, j := range jobs {
		locations[j.Location] = true
	}
	if !locations["Grove, UK"] || !locations["Brackley, UK"] {
		t.Fatalf("locations = %v", locations)
	}
}

func TestSchemaOrgRefusesAnIndexOrUnavailableListing(t *testing.T) {
	f := schemaorgFake{pages: map[string]string{
		"https://careers.example/sitemap.xml": `<sitemapindex><sitemap><loc>https://careers.example/jobs.xml</loc></sitemap></sitemapindex>`,
	}}
	for _, board := range []string{"careers.example/sitemap.xml", "careers.example/missing", "user@careers.example/jobs.xml"} {
		if _, err := NewSchemaOrg(f).Fetch(context.Background(), CompanyEntry{Company: "F1 Team", Board: board}); err == nil {
			t.Errorf("Fetch(%q) succeeded without a usable listing", board)
		}
	}
}

func TestSchemaOrgPartialDetailFailureRemainsUnreadable(t *testing.T) {
	f := schemaorgFake{pages: map[string]string{
		"https://careers.example/jobs.xml":       `<urlset><url><loc>https://careers.example/job/data-jid-1</loc></url><url><loc>https://careers.example/job/data-jid-2</loc></url></urlset>`,
		"https://careers.example/job/data-jid-1": `<html>Temporarily unavailable</html>`,
		"https://careers.example/job/data-jid-2": schemaorgDetailFixture,
	}}
	jobs, err := NewSchemaOrg(f).Fetch(context.Background(), CompanyEntry{Company: "F1 Team", Board: "careers.example/jobs.xml"})
	if err != nil || len(jobs) != 2 {
		t.Fatalf("unreadable detail was dropped or emitted as a real vacancy: %+v, %v", jobs, err)
	}
	unreadable := 0
	for _, j := range jobs {
		if j.Unreadable {
			unreadable++
		}
	}
	if unreadable != 1 {
		t.Fatalf("unreadable postings = %d, want 1", unreadable)
	}
	delete(f.pages, "https://careers.example/job/data-jid-2")
	if _, err := NewSchemaOrg(f).Fetch(context.Background(), CompanyEntry{Company: "F1 Team", Board: "careers.example/jobs.xml"}); err == nil {
		t.Fatal("a total detail outage was reported as a successful empty crawl")
	}
}
