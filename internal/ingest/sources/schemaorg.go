package sources

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

// schemaorg reads a curated career listing or a flat job sitemap, then the public
// JobPosting structured data on each detail page. The board is host/path (without
// a scheme), e.g. racingcareers.mclaren.com/vacanciessitemap.xml. This is for
// custom career sites without a supported ATS feed; it never crawls a whole site.
type schemaorg struct{ http schemaorgHTTP }

type schemaorgHTTP interface {
	HTMLGetter
	XMLGetter
}

// NewSchemaOrg builds the public structured-data career adapter.
func NewSchemaOrg(c schemaorgHTTP) Source { return schemaorg{http: c} }

func (schemaorg) Provider() string { return "schemaorg" }

var schemaorgJobPath = regexp.MustCompile(`(?i)(/jobs?/[^/?#]+|/careers/vacancies/[0-9]+|/[^/?#]*-ref[0-9]+[a-z]*)$`)

func (s schemaorg) Fetch(ctx context.Context, e CompanyEntry) ([]Job, error) {
	base, err := url.Parse("https://" + e.Board)
	if err != nil || base.Hostname() == "" || base.User != nil || base.Fragment != "" {
		return nil, fmt.Errorf("schemaorg: invalid career listing %q", e.Board)
	}
	var links []string
	if strings.HasSuffix(base.Path, ".xml") {
		doc, err := getSitemap(ctx, s.http, base.String())
		if err != nil {
			return nil, fmt.Errorf("schemaorg: sitemap %s: %w", e.Board, err)
		}
		if len(doc.Sitemaps) > 0 {
			return nil, fmt.Errorf("schemaorg: %s is a sitemap index; configure its job sitemap instead", e.Board)
		}
		seen := map[string]bool{}
		for _, item := range doc.URLs {
			if u, ok := schemaorgDetailURL(base, item.Loc); ok && !seen[u] {
				links = append(links, u)
				seen[u] = true
			}
		}
	} else {
		links, err = crawlPagedLinks(ctx, s.http, 100, func(page int) string {
			u := *base
			q := u.Query()
			q.Set("page", fmt.Sprint(page))
			u.RawQuery = q.Encode()
			return u.String()
		}, func(root *xhtml.Node) []string {
			return jobLinks(base, root, func(href string) bool {
				_, ok := schemaorgDetailURL(base, href)
				return ok
			})
		})
		if err != nil {
			return nil, fmt.Errorf("schemaorg: listing %s: %w", e.Board, err)
		}
	}
	clean := make([]string, 0, len(links))
	seen := map[string]bool{}
	for _, raw := range links {
		if normalized, ok := schemaorgDetailURL(base, raw); ok && !seen[normalized] {
			clean = append(clean, normalized)
			seen[normalized] = true
		}
	}
	jobs := fetchDetails(clean, 2, func(raw string) (Job, bool) {
		return s.detail(ctx, e, raw)
	})
	readable := 0
	for _, job := range jobs {
		if !job.Unreadable {
			readable++
		}
	}
	if len(clean) > 0 && readable == 0 {
		return nil, fmt.Errorf("schemaorg: listed %d postings and read none on %s", len(clean), e.Board)
	}
	return jobs, nil
}

// Only same-origin job paths are candidates. Strip tracking parameters before
// hashing the URL so a campaign link cannot mint a second copy of a vacancy.
func schemaorgDetailURL(base *url.URL, raw string) (string, bool) {
	ref, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	u := base.ResolveReference(ref)
	if u.Scheme != "https" || u.Host != base.Host || !schemaorgJobPath.MatchString(strings.TrimRight(u.Path, "/")) {
		return "", false
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String(), true
}

type schemaorgPosting struct {
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	DatePosted      string          `json:"datePosted"`
	EmploymentType  json.RawMessage `json:"employmentType"`
	JobLocationType string          `json:"jobLocationType"`
	JobLocation     schemaPlaces    `json:"jobLocation"`
}

func (s schemaorg) detail(ctx context.Context, e CompanyEntry, raw string) (Job, bool) {
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	root, err := s.http.GetHTML(ctx, raw)
	if err != nil {
		if detailUnreadable(err) {
			return unreadableDetail(id, raw, e.Company), true
		}
		return Job{}, false
	}
	var p schemaorgPosting
	if !ldJobPosting(root, &p) || strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Description) == "" {
		return unreadableDetail(id, raw, e.Company), true
	}
	var locations []string
	for _, place := range p.JobLocation {
		if loc := place.Address.Location(); loc != "" {
			locations = append(locations, loc)
		}
	}
	remote := strings.EqualFold(p.JobLocationType, "TELECOMMUTE")
	mode := ""
	if remote {
		mode = "remote"
	}
	return Job{
		ExternalID: id, URL: raw, Title: html.UnescapeString(p.Title), Company: e.Company,
		Description: sanitizeHTML(html.UnescapeString(p.Description)),
		Location:    strings.Join(locations, "; "), PostedAt: parseRFC3339OrDate(p.DatePosted),
		Remote: remote, WorkMode: mode, EmploymentType: schemaorgEmploymentType(p.EmploymentType),
	}, true
}

func schemaorgEmploymentType(raw json.RawMessage) string {
	var values []string
	if json.Unmarshal(raw, &values) != nil {
		var single string
		if json.Unmarshal(raw, &single) != nil {
			return ""
		}
		values = []string{single}
	}
	for _, value := range values {
		if mapped := schemaEmploymentType(value); mapped != "" {
			return mapped
		}
	}
	return ""
}
