// A saga has no isolation, so the isolation goes in the data. Saga 1 holds the
// last phone for order 501 and later fails. Without a flag, saga 2 reads 0 and
// loses the sale. With a flag, saga 2 waits and gets the phone. A sweeper
// clears the flag of a saga that crashed. Times are minutes past midnight.
package main

import "fmt"

// Row is one stock row. Owner and Since say who set the flag and when.
type Row struct {
	Left   int
	Status string
	Owner  int
	Since  int
}

// PlainReserve commits at once. Everyone sees the new number.
func (r *Row) PlainReserve() { r.Left-- }

// PlainRead returns the number, with nothing to say it is not final.
func (r *Row) PlainRead() int { return r.Left }

// PlainRelease is the compensation: the number goes back up.
func (r *Row) PlainRelease() { r.Left++ }

// Reserve refuses to treat a pending value as final.
func (r *Row) Reserve(saga, now int) string {
	if r.Status == "PENDING" {
		return fmt.Sprintf("wait, pending by %d", r.Owner)
	}
	if r.Left == 0 {
		return "sold out"
	}
	r.Left--
	r.Status, r.Owner, r.Since = "PENDING", saga, now
	return "reserved"
}

// Release is the compensation, and only the owner's flag is cleared.
func (r *Row) Release(saga int) {
	if r.Status != "PENDING" || r.Owner != saga {
		return
	}
	r.Left++
	r.Status, r.Owner = "", 0
}

// Confirm is the final step. A crashed saga runs neither Release nor Confirm.
func (r *Row) Confirm(saga int) {
	r.Status, r.Owner = "SOLD", saga
}

// Sweep undoes a saga whose flag is older than the limit.
func (r *Row) Sweep(now, limit int) bool {
	if r.Status == "PENDING" && now-r.Since >= limit {
		r.Release(r.Owner)
		return true
	}
	return false
}

func main() {
	nine := 9 * 60

	fmt.Println("== No isolation")
	r := &Row{Left: 1}
	r.PlainReserve()
	fmt.Printf("saga 2 reads %d left: told sold out, order 502 cancelled\n", r.PlainRead())
	r.PlainRelease()
	fmt.Printf("saga 1 compensates: %d left, unsold\n", r.PlainRead())

	fmt.Println("== Semantic lock")
	r = &Row{Left: 1}
	fmt.Println("saga 501:", r.Reserve(501, nine))
	fmt.Println("saga 502:", r.Reserve(502, nine+1))
	r.Release(501)
	fmt.Println("saga 502 retries:", r.Reserve(502, nine+2))
	r.Confirm(502)
	fmt.Printf("row: %s to %d, %d left\n", r.Status, r.Owner, r.Left)

	fmt.Println("== A saga crashes holding the flag")
	r = &Row{Left: 1}
	r.Reserve(503, nine)
	fmt.Println("saga 502:", r.Reserve(502, nine+1))
	fmt.Println("sweeper at 09:14 acts:", r.Sweep(nine+14, 15))
	fmt.Println("sweeper at 09:15 acts:", r.Sweep(nine+15, 15))
	fmt.Println("saga 502 retries:", r.Reserve(502, nine+16))
}
