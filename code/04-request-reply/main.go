// Request-reply over messaging: reply_to, correlation_id, the timeout, the
// late reply, and the retried command.
//
// A Go map stands in for the API's table of waiting calls. Replies are
// handed to OnReply in the order the run chooses, so timing is exact.
// Times are in milliseconds and only added up. Deterministic.
package main

import "fmt"

type Reply struct {
	CorrelationID string
	Price         float64
}

type API struct {
	waiting map[string]int // correlation id -> deadline
	late    int
}

// Send records a waiting call with its deadline.
func (a *API) Send(id string, now, timeout int) {
	a.waiting[id] = now + timeout
}

// Request is what the API publishes to price.requests.
type Request struct{ CorrelationID, Token, ReplyTo string }

// queues holds the replies published to each reply queue.
var queues = map[string][]Reply{}

func publish(queue string, r Reply) { queues[queue] = append(queues[queue], r) }

// compute is the worker's price lookup, in this example.
func compute(token string) float64 {
	return map[string]float64{"WETH": 2500, "WBTC": 60000, "USDC": 1}[token]
}

// worker: answer to the queue the request names
func worker(req Request) {
	reply := Reply{CorrelationID: req.CorrelationID, Price: compute(req.Token)}
	publish(req.ReplyTo, reply)
}

// OnReply matches a reply to its waiting call by correlation id.
func (a *API) OnReply(r Reply) string {
	if _, ok := a.waiting[r.CorrelationID]; !ok {
		a.late++ // nobody waits for this one: log and drop
		return "no waiting call: logged and dropped"
	}
	delete(a.waiting, r.CorrelationID)
	return fmt.Sprintf("answered %s: %.0f", r.CorrelationID, r.Price)
}

// Tick expires waiting calls whose deadline has passed.
func (a *API) Tick(now int) []string {
	var gone []string
	for id, d := range a.waiting {
		if now >= d {
			delete(a.waiting, id)
			gone = append(gone, id)
		}
	}
	return gone
}

func main() {
	fmt.Println("== Tab 1: replies out of order ==")
	a := &API{waiting: map[string]int{}}
	for _, id := range []string{"q-41", "q-42", "q-43"} {
		a.Send(id, 0, 2000)
	}
	// the workers answer q-43 before q-42
	worker(Request{"q-41", "WETH", "api.replies"})
	worker(Request{"q-43", "USDC", "api.replies"})
	worker(Request{"q-42", "WBTC", "api.replies"})
	for _, r := range queues["api.replies"] {
		fmt.Println(a.OnReply(r))
	}

	fmt.Println("== Tab 2: a late reply ==")
	a.Send("q-50", 0, 2000)
	fmt.Println("t=2000 timed out:", a.Tick(2000), "page gets: price unavailable")
	fmt.Println("t=5000 worker answers q-50:", a.OnReply(Reply{"q-50", 2500}))

	// A retried command with a new id charges twice.
	charges := map[string]int{}
	charge := func(key string) { charges[key]++ }
	charge("q-50") // the slow worker finishes
	charge("q-51") // the retry, on a second worker
	fmt.Printf("retry with a new id: %d charges\n", len(charges))

	once := map[string]bool{}
	cards := 0
	for _, key := range []string{"order-501", "order-501"} { // same idempotency key
		if !once[key] {
			once[key] = true
			cards++
		}
	}
	fmt.Printf("retry with one idempotency key: %d charge\n", cards)

	expiration, waited := 2000, 3000
	fmt.Printf("expiration %d ms: a request that waited %d ms is dropped unread: %v\n", expiration, waited, waited >= expiration)
}
