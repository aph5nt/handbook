// Command evcache replays the EVCache lesson: one write per zone, local
// reads, a miss that asks another zone and then Cassandra, a write that did
// not land, a cold copy, and warming before traffic.
package main

import "fmt"

type Zone struct {
	Name  string
	Data  map[string]string
	Down  bool // writes time out
	Reads int
}

type Client struct {
	Zones     []*Zone // [A, B, C]
	Cassandra map[string]string
	DBReads   int
}

func (c *Client) Write(key, value string) (failed []string) {
	for _, z := range c.Zones {
		if z.Down {
			failed = append(failed, z.Name)
			continue
		}
		z.Data[key] = value
	}
	return failed
}

// Read runs in zone `me` and falls back to zone `other`.
func (c *Client) Read(me, other *Zone, key string) (string, string) {
	me.Reads++
	if v, ok := me.Data[key]; ok {
		return v, me.Name
	}
	other.Reads++
	if v, ok := other.Data[key]; ok {
		return v, other.Name
	}
	c.DBReads++
	v := c.Cassandra[key]
	me.Data[key] = v
	return v, "Cassandra"
}

func Warm(newCopy, from *Zone) {
	for k, v := range from.Data {
		newCopy.Data[k] = v
	}
}

func main() {
	A := &Zone{Name: "A", Data: map[string]string{}}
	B := &Zone{Name: "B", Data: map[string]string{}}
	C := &Zone{Name: "C", Data: map[string]string{}}
	c := &Client{Zones: []*Zone{A, B, C}, Cassandra: map[string]string{}}
	c.Cassandra["ola"] = "episode 3"
	c.Write("ola", "episode 3")
	fmt.Println("step 1: A, B, C all hold ola = episode 3")

	c.Cassandra["ola"] = "episode 4"
	fmt.Println("step 2: write episode 4, failed zones:", c.Write("ola", "episode 4"))
	v, from := c.Read(A, B, "ola")
	fmt.Printf("step 3: read in zone A -> %s from zone %s\n", v, from)
	c.Cassandra["rare"] = "old film"
	B.Data["rare"] = "old film"
	v, from = c.Read(A, B, "rare")
	fmt.Printf("step 4: rare key, miss in A -> %s from zone %s\n", v, from)

	B.Down = true
	c.Cassandra["ola"] = "episode 5"
	fmt.Println("step 5: write episode 5, failed zones:", c.Write("ola", "episode 5"))
	B.Down = false
	v, _ = c.Read(B, C, "ola")
	fmt.Printf("  a TV in zone B reads ola -> %s (stale)\n", v)

	fmt.Println("step 6: zone A restarts empty; 1000 reads of 1000 hot keys arrive in A")
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("user%d", i)
		c.Cassandra[k] = "row"
		if i >= 10 { // zone B lost 10 keys to expiry
			B.Data[k] = "row"
		}
	}
	A.Data, A.Reads, B.Reads, c.DBReads = map[string]string{}, 0, 0, 0
	for i := 0; i < 1000; i++ {
		c.Read(A, B, fmt.Sprintf("user%d", i))
	}
	fmt.Printf("  zone A answered %d, extra cross-zone reads on B %d, Cassandra %d\n", A.Reads-B.Reads, B.Reads, c.DBReads)

	fmt.Println("step 7: warm a fresh copy from zone B before it takes traffic")
	A.Data, A.Reads, B.Reads, c.DBReads = map[string]string{}, 0, 0, 0
	Warm(A, B)
	for i := 0; i < 1000; i++ {
		c.Read(A, B, fmt.Sprintf("user%d", i))
	}
	fmt.Printf("  zone A answered %d, cross-zone reads on B %d, Cassandra %d\n", A.Reads-B.Reads, B.Reads, c.DBReads)
}
