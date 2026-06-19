package extract

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/four-flames/four-seo-check/runner/internal/model"
	"golang.org/x/net/html"
)

// Links extracts all <a href="..."> elements from an HTML document.
// Relative URLs are resolved against baseURL.
func Links(doc *html.Node, baseURL *url.URL) []model.DiscoveredReference {
	var refs []model.DiscoveredReference
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")
			if href == "" {
				goto skip
			}

			// Skip non-navigable URLs
			trimmed := strings.TrimSpace(href)
			if strings.HasPrefix(trimmed, "javascript:") ||
				strings.HasPrefix(trimmed, "mailto:") ||
				strings.HasPrefix(trimmed, "tel:") ||
				trimmed == "#" ||
				strings.HasPrefix(trimmed, "#") &&
					len(strings.TrimPrefix(trimmed, "#")) > 0 &&
					!strings.HasPrefix(trimmed, "http") {
				// Skip javascript:, mailto:, tel:, and fragment-only links
				if strings.HasPrefix(trimmed, "javascript:") ||
					strings.HasPrefix(trimmed, "mailto:") ||
					strings.HasPrefix(trimmed, "tel:") {
					goto skip
				}
				if trimmed == "#" || (strings.HasPrefix(trimmed, "#") && !strings.Contains(trimmed, "://")) {
					goto skip
				}
			}

			resolved, err := resolveURL(baseURL, trimmed)
			if err != nil || resolved == "" {
				goto skip
			}

			// Only handle http/https
			u, err := url.Parse(resolved)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				goto skip
			}

			anchorText := extractText(n)
			nofollow := hasNofollow(n)
			targetType := model.TargetPage

			// Determine if external
			if u.Host != baseURL.Host {
				targetType = model.TargetExternalLink
			}

			refs = append(refs, model.DiscoveredReference{
				SourceURL:  baseURL.String(),
				TargetURL:  resolved,
				TargetType: targetType,
				AnchorText: anchorText,
				Nofollow:   nofollow,
			})
		}
	skip:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return refs
}

// Images extracts all <img src="..."> elements from an HTML document.
func Images(doc *html.Node, baseURL *url.URL) []model.DiscoveredReference {
	var refs []model.DiscoveredReference
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			src := getAttr(n, "src")
			if src == "" {
				goto skip
			}

			trimmed := strings.TrimSpace(src)
			resolved, err := resolveURL(baseURL, trimmed)
			if err != nil || resolved == "" {
				goto skip
			}

			u, err := url.Parse(resolved)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				goto skip
			}

			altText := strings.TrimSpace(getAttr(n, "alt"))

			refs = append(refs, model.DiscoveredReference{
				SourceURL:  baseURL.String(),
				TargetURL:  resolved,
				TargetType: model.TargetImage,
				AnchorText: altText,
			})
		}
	skip:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return refs
}

// ImageClassMap extracts the CSS class attribute from all <img> elements.
// Returns a map from normalized image URL to its class string.
func ImageClassMap(doc *html.Node, baseURL *url.URL) map[string]string {
	result := make(map[string]string)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			src := getAttr(n, "src")
			if src == "" {
				goto next
			}
			trimmed := strings.TrimSpace(src)
			resolved, err := resolveURL(baseURL, trimmed)
			if err != nil || resolved == "" {
				goto next
			}
			cls := strings.TrimSpace(getAttr(n, "class"))
			result[resolved] = cls
		}
	next:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

// Title extracts the <title> text from an HTML document.
func Title(doc *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = strings.TrimSpace(n.FirstChild.Data)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return title
}

// MetaDescription extracts the content of <meta name="description" ...>.
func MetaDescription(doc *html.Node) string {
	var desc string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			name := strings.ToLower(getAttr(n, "name"))
			if name == "description" {
				desc = strings.TrimSpace(getAttr(n, "content"))
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return desc
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func resolveURL(base *url.URL, href string) (string, error) {
	u, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(u)
	return resolved.String(), nil
}

func extractText(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return strings.TrimSpace(sb.String())
}

func hasNofollow(n *html.Node) bool {
	rel := strings.ToLower(getAttr(n, "rel"))
	if rel == "" {
		return false
	}
	parts := strings.Fields(rel)
	for _, p := range parts {
		if p == "nofollow" {
			return true
		}
	}
	return false
}

// Headings extracts all h1-h6 elements with their text content.
func Headings(doc *html.Node) []model.Heading {
	var headings []model.Heading
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			level := headingLevel(n.Data)
			if level > 0 {
				headings = append(headings, model.Heading{
					Level: level,
					Text:  strings.TrimSpace(extractText(n)),
				})
				return // Don't recurse into headings
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return headings
}

func headingLevel(tag string) int {
	switch tag {
	case "h1":
		return 1
	case "h2":
		return 2
	case "h3":
		return 3
	case "h4":
		return 4
	case "h5":
		return 5
	case "h6":
		return 6
	}
	return 0
}

// CanonicalURL extracts the canonical URL from <link rel="canonical" href="...">
func CanonicalURL(doc *html.Node) string {
	var canonical string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			rel := strings.ToLower(getAttr(n, "rel"))
			if rel == "canonical" {
				canonical = getAttr(n, "href")
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return canonical
}

// RobotsMeta extracts the content of <meta name="robots" content="...">
func RobotsMeta(doc *html.Node) string {
	var robots string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			if strings.ToLower(getAttr(n, "name")) == "robots" {
				robots = getAttr(n, "content")
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return robots
}

// StructuredDataScripts extracts all JSON-LD script blocks.
func StructuredDataScripts(doc *html.Node) []model.StructuredDataItem {
	var items []model.StructuredDataItem
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			typ := strings.ToLower(getAttr(n, "type"))
			if typ == "application/ld+json" {
				item := model.StructuredDataItem{Raw: extractText(n)}
				// Parse JSON to extract @type
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(item.Raw), &data); err == nil {
					if t, ok := data["@type"]; ok {
						switch v := t.(type) {
						case string:
							item.Type = v
						case []interface{}:
							types := make([]string, 0, len(v))
							for _, tv := range v {
								if s, ok := tv.(string); ok {
									types = append(types, s)
								}
							}
							item.Type = strings.Join(types, ", ")
						}
					}
					item.Valid = true
				} else {
					item.Error = err.Error()
				}
				items = append(items, item)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return items
}

// OpenGraphTags extracts Open Graph meta tags.
func OpenGraphTags(doc *html.Node) model.OpenGraph {
	og := model.OpenGraph{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			property := strings.ToLower(getAttr(n, "property"))
			content := getAttr(n, "content")
			switch property {
			case "og:title":
				og.Title = content
			case "og:description":
				og.Description = content
			case "og:image":
				og.Image = content
			case "og:url":
				og.URL = content
			case "og:type":
				og.Type = content
			case "og:site_name":
				og.SiteName = content
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return og
}

// Viewport extracts the viewport meta content.
func Viewport(doc *html.Node) string {
	var vp string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			if strings.ToLower(getAttr(n, "name")) == "viewport" {
				vp = getAttr(n, "content")
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return vp
}

// CharsetMeta extracts charset from meta tag.
func CharsetMeta(doc *html.Node) string {
	var charset string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			if getAttr(n, "charset") != "" {
				charset = getAttr(n, "charset")
				return
			}
			if strings.ToLower(getAttr(n, "http-equiv")) == "content-type" {
				content := getAttr(n, "content")
				if idx := strings.Index(content, "charset="); idx >= 0 {
					charset = content[idx+8:]
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return charset
}

// SrcSetURLs extracts all URLs from srcset attributes on <img> and <source> elements.
func SrcSetURLs(doc *html.Node, baseURL *url.URL) []string {
	var urls []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			srcset := getAttr(n, "srcset")
			if srcset != "" {
				// Parse srcset: "url1 1x, url2 2x" or "url1 400w, url2 800w"
				parts := strings.Split(srcset, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					// Split by whitespace, first part is URL
					fields := strings.Fields(part)
					if len(fields) > 0 {
						resolved, err := resolveURL(baseURL, fields[0])
						if err == nil && resolved != "" {
							urls = append(urls, resolved)
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return urls
}

// SrcSetDescriptors parses srcset attributes on <img> and <source> elements,
// returning structured entries with width and density descriptors.
// Format: "url1 400w, url2 2x" or "url1 400w, url2 800w"
func SrcSetDescriptors(doc *html.Node, baseURL *url.URL) []model.SrcSetEntry {
	var entries []model.SrcSetEntry
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "img" || n.Data == "source") {
			srcset := getAttr(n, "srcset")
			if srcset == "" {
				goto next
			}
			parts := strings.Split(srcset, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				fields := strings.Fields(part)
				if len(fields) == 0 {
					continue
				}
				resolved, err := resolveURL(baseURL, fields[0])
				if err != nil || resolved == "" {
					continue
				}
				entry := model.SrcSetEntry{URL: resolved}
				for _, f := range fields[1:] {
					if strings.HasSuffix(f, "w") {
						if w, err := strconv.Atoi(strings.TrimSuffix(f, "w")); err == nil {
							entry.Width = w
						}
					} else if strings.HasSuffix(f, "x") {
						if d, err := strconv.ParseFloat(strings.TrimSuffix(f, "x"), 64); err == nil {
							entry.Density = d
						}
					}
				}
				entries = append(entries, entry)
			}
		}
	next:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return entries
}

// WordCount estimates word count from visible text.
func WordCount(doc *html.Node) int {
	text := extractText(doc)
	words := strings.Fields(text)
	return len(words)
}
