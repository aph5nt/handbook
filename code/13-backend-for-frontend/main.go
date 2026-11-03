// A phone app and a web app over shared services. Replays the lesson
// 13-backend-for-frontend: one mobile call fans out to three services and
// returns a small card; a rule copied into two BFFs disagrees at the
// threshold; moving it into the pool service ends that.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

const threshold = 1_000_000 // TVL at which the discount starts

// pool is the shared pool service. Its answer is built for the web.
func poolService(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{"name": "WETH/USDC", "fee_bps": 30.0, "tvl_usd": threshold + 0.0,
		"history": strings.Repeat("x", 4000), "fee_after_discount": discount(30, threshold)})
}

func tradersService(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `{"traders":"`+strings.Repeat("y", 8000)+`"}`)
}

func chartService(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{"points": []int{1, 2, 3, 4, 5, 6}, "raw": strings.Repeat("z", 6000)})
}

// discount is the rule in the one place it belongs: the pool service.
func discount(fee, tvl float64) float64 {
	if tvl >= threshold {
		return fee * 0.9
	}
	return fee
}

// mobileBFF fans out in parallel and shapes one answer for one screen.
func mobileBFF(pool, traders, chart string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urls := map[string]string{"pool": pool, "traders": traders, "chart": chart}
		raw := map[string][]byte{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for name, url := range urls {
			wg.Add(1)
			go func() {
				defer wg.Done()
				body := fetch(url)
				mu.Lock()
				raw[name] = body
				mu.Unlock()
			}()
		}
		wg.Wait()
		var p struct {
			Name string  `json:"name"`
			Fee  float64 `json:"fee_after_discount"`
		}
		var c struct {
			Points []int `json:"points"`
		}
		json.Unmarshal(raw["pool"], &p)
		json.Unmarshal(raw["chart"], &c)
		json.NewEncoder(w).Encode(map[string]any{"name": p.Name, "fee": p.Fee, "sparkline": c.Points})
	}
}

func fetch(url string) []byte {
	res, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return body
}

// The rule copied into each BFF: at least on the web, more than on the phone.
func webFee(fee, tvl float64) float64 {
	if tvl >= threshold {
		return fee * 0.9
	}
	return fee
}

func mobileFee(fee, tvl float64) float64 {
	if tvl > threshold {
		return fee * 0.9
	}
	return fee
}

func main() {
	pool := httptest.NewServer(http.HandlerFunc(poolService))
	traders := httptest.NewServer(http.HandlerFunc(tradersService))
	chart := httptest.NewServer(http.HandlerFunc(chartService))
	defer pool.Close()
	defer traders.Close()
	defer chart.Close()

	fmt.Println("--- a thin BFF per client")
	direct := len(fetch(pool.URL)) + len(fetch(traders.URL)) + len(fetch(chart.URL))
	fmt.Println("phone calls the services itself: 3 calls,", direct, "bytes")
	bff := httptest.NewServer(mobileBFF(pool.URL, traders.URL, chart.URL))
	defer bff.Close()
	card := fetch(bff.URL + "/card/pools/42")
	fmt.Println("phone calls the mobile BFF: 1 call,", len(card), "bytes:", strings.TrimSpace(string(card)))

	fmt.Println("--- rules creep into the BFFs")
	for _, tvl := range []float64{threshold - 1, threshold, threshold + 1} {
		fmt.Printf("TVL %.0f: web fee %.0f | phone fee %.0f\n", tvl, webFee(30, tvl), mobileFee(30, tvl))
	}
	fmt.Printf("rule in the pool service, at the threshold: both read %.0f\n", discount(30, threshold))
}
