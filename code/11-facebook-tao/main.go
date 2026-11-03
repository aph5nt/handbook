// Command tao replays the TAO lesson: a read through follower, leader and
// MySQL, a write through the leader, a stale follower in another region, its
// invalidation, and a hot object. The per-follower limit is a model figure.
package main

import "fmt"

type Cache map[string]int

type Region struct {
	Follower Cache
	MySQL    map[string]int
}

type Leader struct {
	Copy    Cache
	Primary *Region
}

func Read(r *Region, l *Leader, key string) (val int, from string) {
	if v, ok := r.Follower[key]; ok {
		return v, "follower"
	}
	if v, ok := l.Copy[key]; ok {
		r.Follower[key] = v
		return v, "leader"
	}
	v := r.MySQL[key]
	l.Copy[key], r.Follower[key] = v, v
	return v, "MySQL"
}

func Write(r *Region, l *Leader, key string, v int) {
	l.Primary.MySQL[key] = v
	l.Copy[key] = v
	r.Follower[key] = v
}

func Replicate(from, to *Region, key string) {
	to.MySQL[key] = from.MySQL[key]
	Invalidate(to, key)
}

func Invalidate(r *Region, key string) {
	delete(r.Follower, key)
}

func FollowersNeeded(reads, limit int) int {
	n := 1
	for reads/n > limit {
		n *= 2
	}
	return n
}

func main() {
	const key = "likes:77"
	eu := &Region{Follower: Cache{key: 2}, MySQL: map[string]int{key: 2, "likes:9": 5}}
	us := &Region{Follower: Cache{key: 2}, MySQL: map[string]int{key: 2}}
	l := &Leader{Copy: Cache{key: 2}, Primary: eu}
	v, from := Read(eu, l, key)
	fmt.Printf("step 2: Ana reads %d from the %s\n", v, from)
	v, from = Read(eu, l, "likes:9")
	fmt.Printf("step 3: an old photo: %d from %s\n", v, from)
	Write(eu, l, key, 3)
	fmt.Printf("step 4: Ana likes: MySQL %d, leader %d\n", eu.MySQL[key], l.Copy[key])
	v, from = Read(eu, l, key)
	fmt.Printf("step 5: Ana reads %d from the %s\n", v, from)
	us2 := &Leader{Copy: Cache{}, Primary: eu}
	v, from = Read(us, us2, key)
	fmt.Printf("step 6: Ben in the US reads %d from the %s (stale)\n", v, from)
	Replicate(eu, us, key)
	fmt.Println("step 7: invalidated: US follower has the key:", func() bool { _, ok := us.Follower[key]; return ok }())
	v, from = Read(us, us2, key)
	fmt.Printf("step 7: Ben reads %d from %s\n", v, from)
	fmt.Printf("step 8: 8,000,000 reads, limit 1,000,000 per follower: %d followers\n", FollowersNeeded(8000000, 1000000))
}
