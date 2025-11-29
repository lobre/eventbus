package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/lobre/eventbus"
)

type Command interface{}

type DepositMoney struct {
	AccountID string
	Amount    int
}

type WithdrawMoney struct {
	AccountID string
	Amount    int
}

type AccountOpened struct {
	AccountID string
	Timestamp time.Time
}

type MoneyDeposited struct {
	AccountID string
	Amount    int
	Timestamp time.Time
}

type MoneyWithdrawn struct {
	AccountID string
	Amount    int
	Timestamp time.Time
}

type pendingEvent struct {
	Type    string
	Payload any
}

type accountSnapshot struct {
	Balance int
	LastID  string
}

type Account struct {
	ID      string
	Balance int

	Pending []pendingEvent
}

func (a *Account) ApplyStored(e eventbus.Event) {
	switch e.Type {
	case "AccountOpened":
		payload := e.Payload.(AccountOpened)
		a.ID = payload.AccountID
	case "MoneyDeposited":
		payload := e.Payload.(MoneyDeposited)
		a.Balance += payload.Amount
	case "MoneyWithdrawn":
		payload := e.Payload.(MoneyWithdrawn)
		a.Balance -= payload.Amount
	}
}

func (a *Account) applyPending(e pendingEvent) {
	switch e.Type {
	case "AccountOpened":
		payload := e.Payload.(AccountOpened)
		a.ID = payload.AccountID
	case "MoneyDeposited":
		payload := e.Payload.(MoneyDeposited)
		a.Balance += payload.Amount
	case "MoneyWithdrawn":
		payload := e.Payload.(MoneyWithdrawn)
		a.Balance -= payload.Amount
	}
	a.Pending = append(a.Pending, e)
}

func (a *Account) Deposit(amount int) error {
	if amount <= 0 {
		return fmt.Errorf("invalid deposit amount: %d must be positive", amount)
	}

	e := pendingEvent{
		Type: "MoneyDeposited",
		Payload: MoneyDeposited{
			AccountID: a.ID,
			Amount:    amount,
			Timestamp: time.Now(),
		},
	}
	a.applyPending(e)
	return nil
}

func (a *Account) Withdraw(amount int) error {
	if amount <= 0 {
		return fmt.Errorf("invalid withdraw amount: %d must be positive", amount)
	}
	if amount > a.Balance {
		return fmt.Errorf("insufficient funds: attempting to withdraw %d from balance %d", amount, a.Balance)
	}

	e := pendingEvent{
		Type: "MoneyWithdrawn",
		Payload: MoneyWithdrawn{
			AccountID: a.ID,
			Amount:    amount,
			Timestamp: time.Now(),
		},
	}
	a.applyPending(e)
	return nil
}

type CommandHandler struct {
	bus       *eventbus.Bus
	snapshots map[string]accountSnapshot
}

func main() {
	bus := eventbus.New()
	handler := &CommandHandler{
		bus:       bus,
		snapshots: make(map[string]accountSnapshot),
	}

	accountID := "account:42"

	// Initialize the account with an open event.
	handler.bootstrap(accountID)

	must(handler.Handle(DepositMoney{AccountID: accountID, Amount: 100}))
	must(handler.Handle(WithdrawMoney{AccountID: accountID, Amount: 40}))
	must(handler.Handle(DepositMoney{AccountID: accountID, Amount: 25}))

	acct, _ := handler.loadAccount(accountID)
	fmt.Printf("account %s balance=%d\n", accountID, acct.Balance)
}

func (h *CommandHandler) Handle(cmd Command) error {
	switch c := cmd.(type) {
	case DepositMoney:
		return h.handleDeposit(c)
	case WithdrawMoney:
		return h.handleWithdraw(c)
	}
	return errors.New("unknown command")
}

func (h *CommandHandler) handleDeposit(cmd DepositMoney) error {
	acct, lastID := h.loadAccount(cmd.AccountID)
	if err := acct.Deposit(cmd.Amount); err != nil {
		return err
	}
	return h.commit(cmd.AccountID, acct, lastID)
}

func (h *CommandHandler) handleWithdraw(cmd WithdrawMoney) error {
	acct, lastID := h.loadAccount(cmd.AccountID)
	if err := acct.Withdraw(cmd.Amount); err != nil {
		return err
	}
	return h.commit(cmd.AccountID, acct, lastID)
}

func (h *CommandHandler) bootstrap(accountID string) {
	_, err := h.bus.Publish(accountID, "AccountOpened", AccountOpened{AccountID: accountID, Timestamp: time.Now()}, h.bus.Start())
	if err != nil {
		log.Fatalf("bootstrap open: %v", err)
	}
}

func (h *CommandHandler) loadAccount(accountID string) (*Account, string) {
	snap := h.snapshots[accountID]
	acct := &Account{ID: accountID, Balance: snap.Balance}
	lastID := snap.LastID

	h.bus.ForEachEvent(eventbus.Query{Topic: accountID, AfterID: lastID}, func(e eventbus.Event) {
		acct.ApplyStored(e)
		lastID = e.ID
	})

	return acct, lastID
}

func (h *CommandHandler) commit(accountID string, acct *Account, lastID string) error {
	for _, pe := range acct.Pending {
		id, err := h.bus.Publish(accountID, pe.Type, pe.Payload, lastID)
		if err != nil {
			return err
		}
		lastID = id
	}
	acct.Pending = nil

	h.snapshots[accountID] = accountSnapshot{
		Balance: acct.Balance,
		LastID:  lastID,
	}
	return nil
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
