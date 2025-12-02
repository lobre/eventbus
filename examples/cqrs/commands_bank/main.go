package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()
	accountTopic := "account-42"

	projection := &balanceProjection{}
	sub, _ := bus.Subscribe(accountTopic, bus.Start())
	defer sub.Close()

	go func() {
		for e := range sub.C {
			projection.Apply(e)
		}
	}()

	handleCommand(bus, accountTopic, command{Name: "Deposit", Amount: 100})
	handleCommand(bus, accountTopic, command{Name: "Withdraw", Amount: 25})
	handleCommand(bus, accountTopic, command{Name: "Deposit", Amount: 50})

	time.Sleep(50 * time.Millisecond)

	fmt.Printf("current balance: %d\n", projection.Value())
}

type command struct {
	Name   string
	Amount int
}

func handleCommand(bus *eventbus.Bus, topic string, cmd command) {
	switch cmd.Name {
	case "Deposit":
		bus.Publish(topic, "Deposited", cmd.Amount, bus.End())

	case "Withdraw":
		balance, id := replayBalance(bus, topic)

		if balance < cmd.Amount {
			fmt.Println("withdraw rejected: insufficient funds")
			return
		}

		bus.Publish(topic, "Withdrawn", cmd.Amount, id)

	default:
		fmt.Printf("unknown command %q ignored\n", cmd.Name)
	}
}

func replayBalance(bus *eventbus.Bus, topic string) (int, string) {
	balance := 0
	id := bus.Start()

	bus.ForEachEvent(eventbus.Query{Topic: topic}, func(e eventbus.Event) {
		amt := e.Payload.(int)
		if e.Type == "Deposited" {
			balance += amt
		}
		if e.Type == "Withdrawn" {
			balance -= amt
		}
		id = e.ID
	})

	return balance, id
}

type balanceProjection struct {
	mu      sync.Mutex
	balance int
}

func (p *balanceProjection) Apply(e eventbus.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	amount := e.Payload.(int)
	switch e.Type {
	case "Deposited":
		p.balance += amount
	case "Withdrawn":
		p.balance -= amount
	}
}

func (p *balanceProjection) Value() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.balance
}
