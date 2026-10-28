package bookings

import "slices"

// The state machine lives only here: one transition table per item kind. Every status change
// goes through CanTransition; nothing else in the code decides which moves are valid.
//
//	service: pending_deposit → confirmed → [in_progress →] completed   (+ no_show, cancelled)
//	product: pending_deposit → confirmed → in_progress → ready → [out_for_delivery →] completed
//	rental:  pending_deposit → confirmed → checked_out → [overdue →] returned → completed
var transitions = map[string]map[string][]string{
	"service": {
		"pending_deposit": {"confirmed", "cancelled", "expired"},
		"confirmed":       {"in_progress", "completed", "cancelled", "no_show"}, // completed: finished without "starting" it
		"in_progress":     {"completed"},
	},
	"product": {
		"pending_deposit":  {"confirmed", "cancelled", "expired"},
		"confirmed":        {"in_progress", "cancelled"},
		"in_progress":      {"ready", "cancelled"},
		"ready":            {"out_for_delivery", "completed"},
		"out_for_delivery": {"completed"},
	},
	"rental": {
		"pending_deposit": {"confirmed", "cancelled", "expired"},
		"confirmed":       {"checked_out", "cancelled", "no_show"},
		"checked_out":     {"returned", "overdue"},
		"overdue":         {"returned", "checked_out"}, // back to checked_out only by extending it
		"returned":        {"completed"},
	},
}

// systemOnly statuses are set by the worker (time-based), never from the panel.
var systemOnly = []string{"expired", "overdue"}

// releasing statuses give back the time slot or the units.
var releasing = []string{"cancelled", "expired", "no_show"}

func CanTransition(kind, from, to string) bool {
	return slices.Contains(transitions[kind][from], to)
}

func ByPeople(to string) bool { return !slices.Contains(systemOnly, to) }

func Releases(status string) bool { return slices.Contains(releasing, status) }

// Next lists the moves a person can make from the current status (for the panel's buttons).
func Next(kind, from string) []string {
	var out []string
	for _, s := range transitions[kind][from] {
		if ByPeople(s) && !backward(from, s) {
			out = append(out, s)
		}
	}
	return out
}

// StillAhead reports whether status `to` is still to come from `from` (reachable by later moves).
// A step that must be done "before X" can still be changed while X is ahead.
func StillAhead(kind, from, to string) bool {
	if from == to {
		return false
	}
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range transitions[kind][cur] {
			if backward(cur, next) {
				continue
			}
			if next == to {
				return true
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// backward: the one move that goes back (an extended overdue rental is "rented" again);
// it's neither a panel button nor progress.
func backward(from, to string) bool { return from == "overdue" && to == "checked_out" }
