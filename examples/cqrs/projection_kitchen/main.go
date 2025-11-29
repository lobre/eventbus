package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lobre/eventbus"
)

type meal string

const (
	mealBurger meal = "burger"
	mealPizza  meal = "pizza"
)

func main() {
	bus := eventbus.New()

	// Seed some orders.
	publishOrder(bus, mealPizza)
	publishOrder(bus, mealBurger)
	publishOrder(bus, mealPizza)

	sub, err := bus.Subscribe("orders", bus.Start())
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	ingredients := newIngredientProjection()
	revenue := newRevenueProjection()

	go func() {
		for e := range sub.C {
			ingredients.apply(e)
			revenue.apply(e)
		}
	}()

	// Live orders.
	publishOrder(bus, mealBurger)
	publishOrder(bus, mealBurger)
	publishOrder(bus, mealPizza)

	time.Sleep(50 * time.Millisecond)

	fmt.Println("Ingredient needs:")
	for ing, count := range ingredients.snapshot() {
		fmt.Printf("%s: %d\n", ing, count)
	}

	fmt.Printf("\nExpected revenue: $%.2f\n", revenue.total())
}

func publishOrder(bus *eventbus.Bus, item meal) {
	_, err := bus.Publish("orders", "order_placed", string(item), bus.End())
	if err != nil {
		log.Fatalf("publish order: %v", err)
	}
}

type ingredientProjection struct {
	mu     sync.Mutex
	counts map[string]int
}

func newIngredientProjection() *ingredientProjection {
	return &ingredientProjection{counts: make(map[string]int)}
}

func (p *ingredientProjection) apply(e eventbus.Event) {
	item := meal(e.Payload.(string))

	p.mu.Lock()
	defer p.mu.Unlock()

	switch item {
	case mealBurger:
		p.counts["steak"] += 1
		p.counts["cheese"] += 1
		p.counts["bun"] += 2

	case mealPizza:
		p.counts["tomato"] += 2
		p.counts["dough"] += 1
		p.counts["cheese"] += 1
	}
}

func (p *ingredientProjection) snapshot() map[string]int {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make(map[string]int, len(p.counts))
	for k, v := range p.counts {
		out[k] = v
	}

	return out
}

type revenueProjection struct {
	mu     sync.Mutex
	amount float64
}

func newRevenueProjection() *revenueProjection {
	return &revenueProjection{}
}

func (p *revenueProjection) apply(e eventbus.Event) {
	item := meal(e.Payload.(string))

	p.mu.Lock()
	defer p.mu.Unlock()

	switch item {
	case mealBurger:
		p.amount += 12.50
	case mealPizza:
		p.amount += 15.00
	}
}

func (p *revenueProjection) total() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.amount
}
