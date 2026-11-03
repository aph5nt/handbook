// A local proxy offers graph_query to a model and pays per call with x402.
// Replays the lesson 13-mcp-tools-x402: a clear tool that gets one paid
// answer, then a vague tool and a looping agent that pays five times.
// The model is a small stand-in function: it does what the description says.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
)

// Tool is what a host lists: the only text the model reads.
type Tool struct {
	Name        string
	Description string
	Pattern     string // the input schema: the shape of address
}

var clearTool = Tool{"graph_query",
	"Anchor on one full address. Read-only. Rows are capped at 100. Error anchor-missing: ask for another address.",
	"^0x[0-9a-f]{40}$"}
var vague = Tool{"graph_query", "Query the graph.", ""}

var payTo = "0x" + strings.Repeat("4a", 20)

// graphAPI charges per call: no proof, no answer.
func graphAPI(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Payment-Proof") == "" {
		w.Header().Set("X-Price", "0.01 USDC")
		w.Header().Set("X-Pay-To", payTo)
		w.WriteHeader(http.StatusPaymentRequired)
		return
	}
	address := r.URL.Query().Get("address")
	switch {
	case !strings.HasPrefix(address, "0x"):
		writeProblem(w, http.StatusBadRequest, "anchor-required")
	case strings.HasSuffix(address, "00"):
		writeProblem(w, http.StatusNotFound, "anchor-missing")
	default:
		json.NewEncoder(w).Encode(map[string]int{"rows": 50000})
	}
}

func writeProblem(w http.ResponseWriter, code int, kind string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"type": kind})
}

// Wallet pays once per idempotency key: the same key reuses its proof.
type Wallet struct {
	spent map[string]bool
}

func (p *Wallet) pay(key string) string {
	if !p.spent[key] {
		p.spent[key] = true // the on-chain payment happens here
	}
	return "proof-" + key
}

// callTool is the proxy: call, pay on 402, retry with proof and the same key.
func callTool(url string, t Tool, arg, key string, maxRows int, w *Wallet) (int, map[string]any) {
	if t.Pattern != "" && !regexp.MustCompile(t.Pattern).MatchString(arg) {
		return 400, map[string]any{"type": "schema: address must be 42 characters"}
	}
	do := func(proof string) *http.Response {
		req, _ := http.NewRequest("POST", url+"?address="+arg, nil)
		req.Header.Set("Idempotency-Key", key)
		if proof != "" {
			req.Header.Set("X-Payment-Proof", proof)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		return res
	}
	res := do("")
	if res.StatusCode == http.StatusPaymentRequired {
		res.Body.Close()
		res = do(w.pay(key))
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	if rows, ok := out["rows"].(float64); ok && maxRows > 0 && int(rows) > maxRows {
		out["rows"], out["capped"] = maxRows, true
	}
	return res.StatusCode, out
}

// model is the stand-in: only the description tells it to anchor.
func model(t Tool, name, address string) string {
	if strings.Contains(t.Description, "Anchor") {
		return address
	}
	return name
}

func main() {
	srv := httptest.NewServer(http.HandlerFunc(graphAPI))
	defer srv.Close()
	address := "0x" + strings.Repeat("ab", 20)

	fmt.Println("--- a clear tool, paid with x402")
	fmt.Println("tool list:", clearTool.Name, "|", clearTool.Description)
	res, _ := http.Post(srv.URL+"?address="+address, "", nil)
	fmt.Println("no proof:", res.StatusCode, res.Header.Get("X-Price"), "to", res.Header.Get("X-Pay-To"))
	res.Body.Close()
	w := &Wallet{spent: map[string]bool{}}
	arg := model(clearTool, "Binance", address)
	code, out := callTool(srv.URL, clearTool, arg, "q1", 100, w)
	fmt.Println("after paying:", code, out, "payments", len(w.spent))
	code, _ = callTool(srv.URL, clearTool, arg, "q1", 100, w)
	fmt.Println("same key again:", code, "payments", len(w.spent))
	noLinks := "0x" + strings.Repeat("ab", 19) + "00"
	code, out = callTool(srv.URL, clearTool, noLinks, "q2", 100, w)
	fmt.Println("address with no links:", code, out["type"])

	fmt.Println("--- a vague tool and a looping agent")
	w = &Wallet{spent: map[string]bool{}}
	arg = model(vague, "Binance", address)
	fmt.Println("model sends:", arg)
	for try := 1; try <= 4; try++ {
		code, out = callTool(srv.URL, vague, arg, fmt.Sprintf("try-%d", try), 0, w)
		fmt.Println("try", try, code, out["type"])
	}
	code, out = callTool(srv.URL, vague, address, "try-5", 0, w)
	fmt.Println("try 5 with the address:", code, "rows", out["rows"], "no cap")
	fmt.Println("wallet paid", len(w.spent), "times for one question")

	fmt.Println("--- the eval")
	for _, t := range []Tool{vague, clearTool} {
		passed := model(t, "Binance", address) == address
		fmt.Printf("description %.30q: anchored call %v\n", t.Description, passed)
	}
}
