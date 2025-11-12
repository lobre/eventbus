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
	bus.ForEachEvent(accountTopic, projection.Apply)

	sub, err := bus.Subscribe(accountTopic, 32)
	if err != nil {
		panic(err)
	}
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
		_ = bus.Publish(eventbus.NewEvent(topic, "Deposited", cmd.Amount))
	case "Withdraw":
		_ = bus.Publish(eventbus.NewEvent(topic, "Withdrawn", cmd.Amount))
	default:
		fmt.Printf("unknown command %q ignored\n", cmd.Name)
	}
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
