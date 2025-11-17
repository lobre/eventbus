package main

import (
	"fmt"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

    recordExpense(bus, 40)
    recordExpense(bus, 30)

    if err := recordExpense(bus, 50); err != nil {
        fmt.Println("third expense rejected:", err)
    }
}

const budget = 100

type expenseState struct {
	total int
}

func recordExpense(bus *eventbus.Bus, amount int) error {
	state := &expenseState{}
	lastID := bus.ForEachEvent("budget", func(e eventbus.Event) {
		state.total += e.Payload.(int)
	})

	if state.total+amount > budget {
		return fmt.Errorf("budget exceeded: total=%d attempted=%d", state.total, amount)
	}

    _, err := bus.Publish("budget", "ExpenseRecorded", lastID, amount)
    return err
}
