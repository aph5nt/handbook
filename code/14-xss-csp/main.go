// XSS and CSP, on a harmless toy payload. Wall 1: html/template escapes.
// Wall 2: a toy browser applies the Content Security Policy (CSP) to every
// script it finds, and reports what it blocks.
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// A toy payload that does nothing: it only proves a script tag got through.
const label = `<script>report("page data")</script>`

// pageBroken builds HTML from strings, so the label becomes a real tag.
func pageBroken(label string) string {
	return "<h1>Address 0xabc</h1><p>" + label + "</p>"
}

var pageTmpl = template.Must(template.New("p").Parse(`<h1>Address 0xabc</h1><p>{{.}}</p>`))

// pageFixed uses a template that escapes by default.
func pageFixed(label string) string {
	var b bytes.Buffer
	pageTmpl.Execute(&b, label)
	return b.String()
}

// oldWidget is the missed spot: it writes raw HTML, as innerHTML would.
func oldWidget(page, label string) string { return page + "<div>" + label + "</div>" }

// pagePolicy is the CSP header the page sends: scripts from this site, or
// scripts holding this page's random nonce.
const pagePolicy = "script-src 'self' 'nonce-r4nd0m'"

var scriptTag = regexp.MustCompile(`<script( nonce="([^"]*)")?>`)

// browserRun is the toy browser: for each script tag, the CSP decides.
func browserRun(html, csp, nonce string, reports *[]string) int {
	ran := 0
	for _, m := range scriptTag.FindAllStringSubmatch(html, -1) {
		switch {
		case csp == "" || strings.Contains(csp, "'unsafe-inline'"):
			ran++
		case nonce != "" && m[2] == nonce:
			ran++
		default:
			*reports = append(*reports, "blocked inline script, no valid nonce")
		}
	}
	return ran
}

func main() {
	var reports []string
	fmt.Println("Tab 1: pasted into the page")
	page := pageBroken(label)
	fmt.Println("  html:", page)
	fmt.Println("  scripts that run, no CSP:", browserRun(page, "", "", &reports))

	fmt.Println("Tab 2: two walls")
	safe := pageFixed(label)
	fmt.Println("  escaped html:", safe)
	fmt.Println("  scripts that run:", browserRun(safe, "", "", &reports))
	csp := pagePolicy
	own := `<script nonce="r4nd0m">app()</script>`
	mixed := oldWidget(safe+own, label)
	fmt.Println("  old widget adds the raw label, with CSP:", browserRun(mixed, csp, "r4nd0m", &reports), "script runs (the page's own)")
	fmt.Println("  CSP report:", reports)
	fmt.Println("  policy with 'unsafe-inline':", browserRun(mixed, "script-src 'self' 'unsafe-inline'", "", &reports), "scripts run")
}
