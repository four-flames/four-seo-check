package extract

import (
	"net/url"
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
