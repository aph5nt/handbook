// The browser cache: memory first, then disk, then the network. Replays a
// page load over days, and the deploy where new HTML meets an old app.js
// on disk. Then the fix: a hash of the content in the file name.
package main

import "fmt"

const day = 86400 // seconds

type file struct {
	body   string
	maxAge int
	etag   string
}

type entry struct {
	f      file
	stored int
}

type Browser struct {
	memory map[string]entry // one tab, gone when the tab closes
	disk   map[string]entry // survives restarts
	server map[string]file
	now    int
}

func (b *Browser) store(url string, f file) {
	e := entry{f, b.now}
	b.memory[url], b.disk[url] = e, e
}

// Get walks memory, then disk, then the network, and says where the copy came from.
func (b *Browser) Get(url string) (string, string) {
	if e, ok := b.memory[url]; ok && b.now-e.stored < e.f.maxAge {
		return e.f.body, "memory cache"
	}
	if e, ok := b.disk[url]; ok {
		if b.now-e.stored < e.f.maxAge { // fresh: nobody is asked
			b.memory[url] = e
			return e.f.body, "disk cache, no request"
		}
		if s := b.server[url]; s.etag == e.f.etag { // stale: ask with the ETag
			b.store(url, e.f)
			return e.f.body, "304, kept the copy"
		}
	}
	s := b.server[url] // nothing usable: full download
	b.store(url, s)
	return s.body, "200, downloaded"
}

func newBrowser(server map[string]file) *Browser {
	return &Browser{memory: map[string]entry{}, disk: map[string]entry{}, server: server}
}

func show(b *Browser, url string) string {
	body, from := b.Get(url)
	fmt.Printf("  %-16s %-22s %s\n", url, body, from)
	return body
}

func main() {
	const year = 365 * day
	fmt.Println("== a page load ==")
	b := newBrowser(map[string]file{
		"/page":        {"page", 0, "p1"}, // no-cache: max-age 0, so always checked
		"/app.3f9a.js": {"script v1", year, "s1"},
	})
	fmt.Println(" first visit:")
	show(b, "/page")
	show(b, "/app.3f9a.js")
	fmt.Println(" next page in the same tab:")
	show(b, "/app.3f9a.js")
	b.memory = map[string]entry{} // a new tab
	b.now += day
	fmt.Println(" next day, new tab:")
	show(b, "/page")
	show(b, "/app.3f9a.js")

	fmt.Println("== a deploy with a fixed file name ==")
	s := map[string]file{
		"/page":   {"page calls f1", 0, "p1"},
		"/app.js": {"script with f1", year, "s1"},
	}
	b = newBrowser(s)
	b.Get("/page")
	b.Get("/app.js")
	b.memory = map[string]entry{}
	b.now += day
	s["/page"] = file{"page calls f2", 0, "p2"}
	s["/app.js"] = file{"script with f2", year, "s2"}
	show(b, "/page")
	show(b, "/app.js")
	fmt.Println("  -> the new page calls f2, the old script has only f1: BROKEN")

	fmt.Println("== the fix: hashed names, HTML no-cache ==")
	s["/page"] = file{"page names app.7c21.js", 0, "p3"}
	s["/app.7c21.js"] = file{"script with f2", year, "s3"}
	show(b, "/page")
	show(b, "/app.7c21.js")
}
