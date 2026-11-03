// Local or shared: ten instances cache the page size setting. Local only
// breaks read-your-own-write. Local in front of Redis, with a drop message
// and a short local TTL, keeps the copies close.
package main

import "fmt"

type entry struct {
	val     int
	expires int
}

type Instance struct {
	name  string
	local map[string]entry
}

type Cluster struct {
	db        map[string]int
	redis     map[string]int // nil: no shared layer
	redisDown bool
	localTTL  int
	instances []*Instance
	publish   bool
	lost      map[string]bool // instances that miss the drop message
	dbReads   int
}

// Read walks local, then Redis, then the database.
func (c *Cluster) Read(in *Instance, key string, now int) (int, string) {
	if e, ok := in.local[key]; ok && now < e.expires {
		return e.val, "local"
	}
	if c.redis != nil && !c.redisDown {
		if v, ok := c.redis[key]; ok {
			in.local[key] = entry{v, now + c.localTTL}
			return v, "redis"
		}
	}
	c.dbReads++
	v := c.db[key]
	in.local[key] = entry{v, now + c.localTTL}
	return v, "database"
}

// Save writes the database, its own copy, Redis, and publishes a drop message.
func (c *Cluster) Save(in *Instance, key string, v, now int) {
	c.db[key] = v
	in.local[key] = entry{v, now + c.localTTL}
	if c.redis != nil {
		c.redis[key] = v
	}
	if c.publish {
		for _, other := range c.instances {
			if other != in && !c.lost[other.name] {
				delete(other.local, key) // drop page size
			}
		}
	}
}

func cluster(localTTL int, shared, publish bool) *Cluster {
	c := &Cluster{db: map[string]int{"page size": 50}, localTTL: localTTL, publish: publish, lost: map[string]bool{}}
	if shared {
		c.redis = map[string]int{"page size": 50}
	}
	for i := 1; i <= 10; i++ {
		in := &Instance{fmt.Sprint("Instance ", i), map[string]entry{}}
		c.instances = append(c.instances, in)
		c.Read(in, "page size", 0)
	}
	return c
}

func main() {
	const key = "page size"
	fmt.Println("== local caches only, TTL 30 s ==")
	c := cluster(30, false, false)
	i3, i7 := c.instances[2], c.instances[6]
	c.Save(i3, key, 100, 5)
	v, from := c.Read(i7, key, 6)
	fmt.Printf("  saved 100 on Instance 3. Next page on Instance 7: %d from %s\n", v, from)
	v, from = c.Read(i7, key, 31)
	fmt.Printf("  at second 31, its TTL over: %d from %s\n", v, from)
	i7.local = map[string]entry{}
	before := c.dbReads
	c.Read(i7, key, 40)
	fmt.Printf("  a restart empties it: %d database read\n", c.dbReads-before)

	fmt.Println("== local TTL 5 s in front of Redis, drop message on save ==")
	c = cluster(5, true, true)
	i3, i7 = c.instances[2], c.instances[6]
	c.Save(i3, key, 100, 1)
	v, from = c.Read(i7, key, 2)
	fmt.Printf("  saved 100. Next page on Instance 7: %d from %s\n", v, from)

	c = cluster(5, true, true)
	i3, i7 = c.instances[2], c.instances[6]
	c.lost["Instance 7"] = true
	c.Save(i3, key, 100, 1)
	v, from = c.Read(i7, key, 2)
	fmt.Printf("  drop message lost: at second 2 Instance 7 says %d from %s\n", v, from)
	v, from = c.Read(i7, key, 5)
	fmt.Printf("  at second 5, the short TTL ends it: %d from %s\n", v, from)

	c.redisDown = true
	i7.local = map[string]entry{}
	v, from = c.Read(i7, key, 6)
	fmt.Printf("  Redis down: %d from %s, slower but the page loads\n", v, from)
}
