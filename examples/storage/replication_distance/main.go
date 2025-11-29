package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	seed := eventbus.New()

	// seed with a couple of past runs
	recordActivity(seed, 5.2)
	recordActivity(seed, 3.8)

	remote := newHTTPMock(seed)
	client := &http.Client{Transport: remote}
	url := "http://mock/"

	bus := fetchBusFromURL(client, url)
	fmt.Printf("km after remote load: %.1f\n", totalKm(bus))

	sub, err := bus.Subscribe("distance", bus.End())
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	go replicate(bus, client, url, sub.C)

	for _, km := range []float64{4.0, 6.5, 2.3, 5.7} {
		recordActivity(bus, km)
		fmt.Printf("added a run of %.1f km (total now %.1f)\n", km, totalKm(bus))
	}

	time.Sleep(50 * time.Millisecond)

	fmt.Println("remote log contents:")
	fmt.Println(string(remote.snapshot()))
}

func recordActivity(bus *eventbus.Bus, km float64) {
	_, err := bus.Publish("distance", "run_recorded", km, bus.End())
	if err != nil {
		log.Fatalf("publish activity: %v", err)
	}
}

func totalKm(bus *eventbus.Bus) float64 {
	sum := 0.0
	bus.ForEachEvent(eventbus.Query{Topic: "distance", Type: "run_recorded"}, func(e eventbus.Event) {
		sum += e.Payload.(float64)
	})
	return sum
}

func replicate(bus *eventbus.Bus, client *http.Client, url string, ch <-chan eventbus.Event) {
	count := 0
	for range ch {
		count++
		if count%2 != 0 {
			continue
		}

		// replicate every 2 events
		patchBusToURL(bus, client, url)
	}
}

func fetchBusFromURL(client *http.Client, url string) *eventbus.Bus {
	resp, err := client.Get(url)
	if err != nil {
		log.Fatalf("load remote: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("load status %d: %s", resp.StatusCode, string(body))
	}

	bus := eventbus.New()
	if err := bus.Load(resp.Body); err != nil {
		log.Fatalf("load bus: %v", err)
	}

	return bus
}

func patchBusToURL(bus *eventbus.Bus, client *http.Client, url string) {
	var buf bytes.Buffer
	if err := bus.Dump(&buf); err != nil {
		log.Fatalf("dump: %v", err)
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(buf.Bytes()))
	if err != nil {
		log.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("replicate do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("replicate status %d: %s", resp.StatusCode, string(body))
	}
	fmt.Println("replicated to remote")
}
