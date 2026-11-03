// Command whatsapp replays the relay lesson: one process per connection, a
// queue only for messages in flight, delete on delivery, a crash that stays
// local, and the laptop that finds nothing on the server.
package main

import "fmt"

type Server struct {
	Process  map[string]bool
	Queue    map[string][]string
	Restarts int
}

type Phone struct {
	Online  bool
	History []string
}

func (s *Server) Connect(user string) []string {
	s.Process[user] = true
	return s.Queue[user]
}

func (s *Server) Send(msg, to string) string {
	if s.Process[to] {
		s.Queue[to] = append(s.Queue[to], msg)
		return "handed to " + to + "'s process"
	}
	s.Queue[to] = append(s.Queue[to], msg)
	return to + " offline: queued"
}

func (s *Server) Ack(user string) {
	delete(s.Queue, user)
}

func (s *Server) Crash(user string) {
	delete(s.Process, user)
	s.Restarts++
	s.Process[user] = true
}

func (s *Server) History(user string) []string {
	return s.Queue[user]
}

func LaptopSees(s *Server, user string, phone *Phone) []string {
	if len(s.History(user)) > 0 {
		return s.History(user)
	}
	if phone.Online {
		return phone.History
	}
	return nil
}

func main() {
	s := &Server{Process: map[string]bool{}, Queue: map[string][]string{}}
	s.Connect("Ana")
	fmt.Println("step 1: processes:", len(s.Process))
	fmt.Println("step 2-3:", s.Send("See you at 8", "Ben"), "; queue:", s.Queue["Ben"])
	fmt.Println("step 4: Ben connects, gets:", s.Connect("Ben"))
	s.Ack("Ben")
	fmt.Println("step 5: after the ack, queue length:", len(s.Queue["Ben"]))
	s.Connect("Cara")
	s.Crash("Ben")
	fmt.Println("step 6: Ben's process crashed, restarts:", s.Restarts, "; Ana alive:", s.Process["Ana"], "; Cara alive:", s.Process["Cara"])

	phone := &Phone{Online: true}
	for i := 1; i <= 1200; i++ {
		phone.History = append(phone.History, fmt.Sprintf("message %d", i))
	}
	fmt.Println("laptop tab: server holds", len(s.History("Ben")), "of Ben's", len(phone.History), "messages")
	fmt.Println("laptop tab: phone online, laptop shows", len(LaptopSees(s, "Ben", phone)))
	phone.Online = false
	fmt.Println("laptop tab: phone off, laptop shows", len(LaptopSees(s, "Ben", phone)))
}
