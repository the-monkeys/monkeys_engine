package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var (
		baseURLStr string
		workers    int
		duration   time.Duration
	)

	flag.StringVar(&baseURLStr, "url", "http://localhost:8080/api/v2/blog/meta-feed", "Target URL")
	flag.IntVar(&workers, "workers", 50, "Number of concurrent workers")
	flag.DurationVar(&duration, "duration", 10*time.Second, "Test duration")
	flag.Parse()

	fmt.Printf("Starting load test on %s\n", baseURLStr)
	fmt.Printf("Workers: %d, Duration: %s\n", workers, duration)

	var (
		totalReqs  int64
		totalErrs  int64
		totalBytes int64
		latencies  = make(chan time.Duration, 100000)
		wg         sync.WaitGroup
	)

	start := time.Now()
	done := make(chan struct{})

	// Timer to stop test
	go func() {
		time.Sleep(duration)
		close(done)
	}()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}

			for {
				select {
				case <-done:
					return
				default:
					reqStart := time.Now()
					resp, err := client.Get(baseURLStr)
					latency := time.Since(reqStart)

					if err != nil {
						atomic.AddInt64(&totalErrs, 1)
						continue
					}

					// Read body to emulate real client
					n, _ := io.Copy(io.Discard, resp.Body)
					resp.Body.Close()

					atomic.AddInt64(&totalReqs, 1)
					atomic.AddInt64(&totalBytes, n)

					// Non-blocking send or drop if channel full (sampling)
					select {
					case latencies <- latency:
					default:
					}
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	close(latencies)

	// Calculate Stats
	rps := float64(totalReqs) / elapsed.Seconds()
	avgLatency := time.Duration(0)
	count := 0

	for l := range latencies {
		avgLatency += l
		count++
	}
	if count > 0 {
		avgLatency = time.Duration(int64(avgLatency) / int64(count))
	}

	result := map[string]interface{}{
		"total_requests": totalReqs,
		"total_errors":   totalErrs,
		"elapsed_time":   elapsed.String(),
		"rps":            rps,
		"avg_latency":    avgLatency.String(),
		"throughput_mb":  float64(totalBytes) / 1024 / 1024,
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonBytes))
}
