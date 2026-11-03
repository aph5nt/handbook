// An in-memory model of MVCC row versions with xmin and xmax, and a vacuum that
// applies the real rule: a version is removable only if the transaction that
// replaced it is older than the oldest open snapshot. It needs no database.
package main

import "fmt"

type Version struct {
	Balance    int
	Xmin, Xmax int // created by, replaced by. Xmax 0 means live
	Free       bool
}

type Table struct {
	slots  []Version
	nextTx int
	open   []int // snapshots of open transactions
}

func (t *Table) Update(b int) {
	tx := t.nextTx
	t.nextTx++
	for i := range t.slots {
		if !t.slots[i].Free && t.slots[i].Xmax == 0 {
			t.slots[i].Xmax = tx
		}
	}
	v := Version{Balance: b, Xmin: tx}
	for i := range t.slots {
		if t.slots[i].Free {
			t.slots[i] = v // reuse freed space: the table stops growing
			return
		}
	}
	t.slots = append(t.slots, v)
}

// horizon is the oldest snapshot still open, or the next transaction id.
func (t *Table) horizon() int {
	h := t.nextTx
	for _, s := range t.open {
		if s < h {
			h = s
		}
	}
	return h
}

func (t *Table) Vacuum() (freed int) {
	h := t.horizon()
	for i, v := range t.slots {
		if !v.Free && v.Xmax != 0 && v.Xmax < h {
			t.slots[i].Free = true
			freed++
		}
	}
	return freed
}

func (t *Table) Show(label string) {
	live, dead, free := 0, 0, 0
	for _, v := range t.slots {
		switch {
		case v.Free:
			free++
		case v.Xmax == 0:
			live++
		default:
			dead++
		}
	}
	fmt.Printf("%-34s %d slots: %d live, %d dead, %d free (horizon %d)\n", label, len(t.slots), live, dead, free, t.horizon())
}

func main() {
	a := &Table{slots: []Version{{Balance: 10, Xmin: 100}}, nextTx: 101}
	a.Update(12)
	a.Update(15)
	a.Show("two updates:")
	fmt.Printf("vacuum freed %d\n", a.Vacuum())
	a.Show("after vacuum:")
	a.Update(18)
	a.Show("next update reuses a slot:")

	b := &Table{slots: []Version{{Balance: 10, Xmin: 100}}, nextTx: 101, open: []int{101}}
	for k := 1; k <= 7; k++ {
		b.Update(10 + 3*k)
	}
	b.Show("open snapshot 101, seven updates:")
	fmt.Printf("vacuum freed %d\n", b.Vacuum())
	b.open = nil
	b.Show("the idle session is ended:")
	fmt.Printf("vacuum freed %d\n", b.Vacuum())
	b.Show("after vacuum:")
}
