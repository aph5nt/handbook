// Batch, micro-batch and stream: when does a transfer made at 14:05 become
// visible? And what goes wrong when the daily report is built as a stream?
package main

import "fmt"

// Times are seconds after midnight.
func hhmmss(s int) string { return fmt.Sprintf("%02d:%02d:%02d", s/3600%24, s/60%60, s%60) }

const (
	transferAt  = 14*3600 + 5*60 // 14:05:00
	askAt       = 14*3600 + 6*60 // 14:06:00
	batchRunAt  = 26 * 3600      // 02:00 the next day
	microPeriod = 3              // the micro-batch loop takes a new range every 3 s
)

// visibleBatch: a bounded set on a schedule. The next run after the event.
func visibleBatch(event int) int { return batchRunAt }

// visibleMicro: the range holding the event closes at the next loop tick.
func visibleMicro(event int) int { return (event/microPeriod + 1) * microPeriod }

// visibleStream: each event is handled as it arrives.
func visibleStream(event int) int { return event }

// streamReport counts one transfer per minute of the day, with a restart at
// 15:00. The checkpoint was saved at 14:50, but the state was saved at 14:59:
// state and checkpoint do not match, so 14:50 to 14:59 are counted twice.
func streamReport(stateMatchesCheckpoint bool) int {
	count := 0
	checkpoint := 0
	for m := 0; m < 24*60; m++ {
		if m%10 == 0 {
			checkpoint = m // position saved every 10 minutes
		}
		count++           // dayState[day] += 1, held open all day
		if m == 15*60-1 { // the job restarts at 15:00
			if stateMatchesCheckpoint {
				continue // state and position were saved together
			}
			// state already holds 14:50 to 14:59, the replay adds them again
			count += m - checkpoint + 1
		}
	}
	return count
}

// batchReport is report(day) = count of the day's rows, rerun by date.
func batchReport(day []int) int { return len(day) }

func main() {
	fmt.Printf("transfer at %s, user asks at %s\n", hhmmss(transferAt), hhmmss(askAt))
	for _, r := range []struct {
		name string
		at   int
	}{{"batch", visibleBatch(transferAt)}, {"micro-batch", visibleMicro(transferAt)}, {"stream", visibleStream(transferAt)}} {
		fmt.Printf("  %-11s visible at %s, in time for the question: %v\n", r.name, hhmmss(r.at), r.at <= askAt)
	}
	ok := visibleMicro(transferAt) <= askAt // 14:05:03: true
	fmt.Println("micro-batch answer in time for the question:", ok)
	day := make([]int, 24*60)
	fmt.Printf("daily report, stream with a mismatched restart: %d (true count %d)\n", streamReport(false), len(day))
	fmt.Printf("daily report, nightly batch rerun by date: %d\n", batchReport(day))
}
