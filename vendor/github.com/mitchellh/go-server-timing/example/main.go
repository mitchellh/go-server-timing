package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/mitchellh/go-server-timing"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	// Our handler. In a real application this might be your root router,
	// or some subset of your router. Wrapping this ensures that all routes
	// handled by this handler have access to the server timing header struct.
	var h http.Handler = http.HandlerFunc(handler)

	// Wrap our handler with the server timing middleware
	h = servertiming.Middleware(h, nil)

	// Let the user know what to do for this example
	println("Visit http://127.0.0.1:8080")

	// Start!
	http.ListenAndServe(":8080", h)
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Get our timing header builder from the context
	timing := servertiming.FromContext(r.Context())

	// Imagine your handler performs some tasks in a goroutine, such as
	// accessing some remote service. timing is concurrency safe so we can
	// record how long that takes. Let's simulate making 5 concurrent requests
	// to various servicse.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		name := fmt.Sprintf("service-%d", i)
		go func(name string) {
			// This creats a new metric and starts the timer. The Stop is
			// deferred so when the function exits it'll record the duration.
			defer timing.NewMetric(name).Start().Stop()
			time.Sleep(random(25, 75))
			wg.Done()
		}(name)
	}

	// Imagine this is just some blocking code in your main handler such
	// as a SQL query. Let's record that.
	m := timing.NewMetric("sql").WithDesc("SQL query").Start()
	time.Sleep(random(20, 50))
	m.Stop()

	// Wait for the goroutine to end
	wg.Wait()

	// You could continue recording more metrics, but let's just return now
	w.WriteHeader(200)
	w.Write([]byte("Done. Check your browser inspector timing details."))
}

func random(min, max int) time.Duration {
	return (time.Duration(rand.Intn(max-min) + min)) * time.Millisecond
}
