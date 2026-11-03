// Forged requests, on toy servers only. CSRF: a toy browser decides when it
// attaches the bank cookie; the bank checks SameSite and a form token.
// SSRF: a webhook guard resolves names (toy DNS) and checks every hop.
// CORS: the API names allowed origins instead of echoing the caller's.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
)

// ---------- CSRF ----------

type cookie struct{ value, sameSite string }

// browserSends is the browser rule: a cross-site POST carries the cookie
// only when SameSite is None.
func browserSends(c cookie, crossSite bool) bool {
	return !crossSite || c.sameSite == "None"
}

const formToken = "tok-5f1e" // random per session, printed into the bank's own form

// transfer is the bank handler: a session, then the form token.
func transfer(checkToken bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err != nil || c.Value != "s-42" {
			http.Error(w, "not signed in", http.StatusUnauthorized)
			return
		}
		if checkToken && r.FormValue("csrf") != formToken {
			http.Error(w, "token missing", http.StatusForbidden)
			return
		}
		fmt.Fprint(w, "transfer done")
	}
}

func post(srv *httptest.Server, c cookie, crossSite bool, form url.Values) string {
	req, _ := http.NewRequest("POST", srv.URL+"/transfer", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if browserSends(c, crossSite) {
		req.AddCookie(&http.Cookie{Name: "session", Value: c.value})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	return resp.Status
}

// ---------- SSRF ----------

var toyDNS = map[string]string{"hooks.evil.test": "169.254.169.254", "public.example": "93.184.215.14"}
var toyWeb = map[string]string{"http://public.example/hook": "302 http://hooks.evil.test/latest/meta-data"}

// nameOnly is the broken guard: it checks the hostname text.
func nameOnly(raw string) bool {
	u, _ := url.Parse(raw)
	return u.Hostname() != "localhost"
}

// resolvedOK is the fixed guard: resolve, then refuse private, loopback
// and link-local addresses, which include 169.254.169.254.
func resolvedOK(raw string) (bool, string) {
	u, _ := url.Parse(raw)
	ip, err := netip.ParseAddr(toyDNS[u.Hostname()])
	if err != nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false, toyDNS[u.Hostname()]
	}
	return true, ip.String()
}

// fetchHook follows redirects and runs the guard on every hop.
func fetchHook(raw string, guard func(string) (bool, string)) string {
	for hop := 0; hop < 5; hop++ {
		ok, ip := guard(raw)
		if !ok {
			return fmt.Sprintf("blocked at hop %d: %s is %s", hop+1, raw, ip)
		}
		next, isRedirect := strings.CutPrefix(toyWeb[raw], "302 ")
		if !isRedirect {
			return "fetched " + raw
		}
		raw = next
	}
	return "too many redirects"
}

// ---------- CORS ----------

var allowed = map[string]bool{"https://partner.example": true}

func corsAPI(reflectAny bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if reflectAny || allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		fmt.Fprint(w, `{"name":"victim"}`)
	}
}

func allowHeader(srv *httptest.Server, origin string) string {
	req, _ := http.NewRequest("GET", srv.URL+"/me", nil)
	req.Header.Set("Origin", origin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	if h := resp.Header.Get("Access-Control-Allow-Origin"); h != "" {
		return "Allow-Origin: " + h + " -> browser lets the page read"
	}
	return "no Allow-Origin -> browser blocks the read"
}

func main() {
	fmt.Println("CSRF")
	open := httptest.NewServer(transfer(false))
	defer open.Close()
	guarded := httptest.NewServer(transfer(true))
	defer guarded.Close()
	evilForm := url.Values{"to": {"attacker"}, "amount": {"500"}}
	fmt.Println("  SameSite=None, evil form   ->", post(open, cookie{"s-42", "None"}, true, evilForm))
	fmt.Println("  SameSite=Lax, evil form    ->", post(open, cookie{"s-42", "Lax"}, true, evilForm))
	fmt.Println("  token check, evil form     ->", post(guarded, cookie{"s-42", "None"}, true, evilForm))
	ownForm := url.Values{"to": {"landlord"}, "amount": {"500"}, "csrf": {formToken}}
	fmt.Println("  token check, bank's form   ->", post(guarded, cookie{"s-42", "Lax"}, false, ownForm))

	fmt.Println("SSRF")
	fmt.Println("  name-only guard, hooks.evil.test ->", nameOnly("http://hooks.evil.test/latest/meta-data"), "(then fetches", toyDNS["hooks.evil.test"]+")")
	fmt.Println("  resolve guard, hooks.evil.test   ->", fetchHook("http://hooks.evil.test/latest/meta-data", resolvedOK))
	fmt.Println("  resolve guard, public redirect   ->", fetchHook("http://public.example/hook", resolvedOK))

	fmt.Println("CORS")
	lazy := httptest.NewServer(corsAPI(true))
	defer lazy.Close()
	named := httptest.NewServer(corsAPI(false))
	defer named.Close()
	fmt.Println("  named list, partner ->", allowHeader(named, "https://partner.example"))
	fmt.Println("  reflect any, evil   ->", allowHeader(lazy, "https://evil.example"))
	fmt.Println("  named list, evil    ->", allowHeader(named, "https://evil.example"))
}
