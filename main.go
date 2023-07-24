package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

var (
	fetchDelay           time.Duration
	exitCount            int
	expectedTimeDuration time.Duration
	startTime            time.Time
	successCount         int
	url                  string
	forever              bool
)

func elapsedTime() string {
	duration := time.Since(startTime)

	hours := int64(duration.Hours())
	minutes := int64(duration.Minutes()) % 60
	seconds := int64(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%6s", fmt.Sprintf("%dh%dm", hours, minutes))
	}

	if minutes > 0 {
		return fmt.Sprintf("%6s", fmt.Sprintf("%dm%ds", minutes, seconds))
	}

	return fmt.Sprintf("%6s", fmt.Sprintf("%ds", seconds))
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func formatDuration(duration time.Duration, suffix string) string {
	minutes := int64(duration.Minutes())
	seconds := int64(duration.Seconds()) % 60

	if minutes != 0 {
		return fmt.Sprintf("%6s", fmt.Sprintf("%dm%ds %s", abs(minutes), abs(seconds), suffix))
	}
	return fmt.Sprintf("%6s", fmt.Sprintf("%ds %s", abs(seconds), suffix))
}

func remainingTime() string {
	remaining := expectedTimeDuration - time.Since(startTime)

	suffix := "remaining"
	if remaining < 0 {
		suffix = "ago"
	}

	return formatDuration(remaining, suffix)
}

func fetch(url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("%s (%s) Error creating request: %v\n", elapsedTime(), remainingTime(), err)
		return
	}

	var untrustedCertError error
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				VerifyConnection: func(cs tls.ConnectionState) error {
					opts := x509.VerifyOptions{
						DNSName: cs.ServerName,
						Roots:   x509.NewCertPool(),
					}

					if _, err := cs.PeerCertificates[0].Verify(opts); err != nil {
						untrustedCertError = fmt.Errorf("untrusted SSL certificate: %v", err)
					}

					return nil
				},
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%s (%s) Error: %v\n", elapsedTime(), remainingTime(), err)
		return
	}
	defer resp.Body.Close()

	if untrustedCertError != nil {
		fmt.Printf("%s (%s) HTTP Response Code: %d, %v\n", elapsedTime(), remainingTime(), resp.StatusCode, untrustedCertError)
	} else {
		fmt.Printf("%s (%s) HTTP Response Code: %d\n", elapsedTime(), remainingTime(), resp.StatusCode)
	}

	// Increment successCount if response code is 200 and exit if successCount reached 5
	if resp.StatusCode == http.StatusOK {
		successCount++
		if !forever && successCount == exitCount {
			fmt.Printf("Exiting after %d successful fetches.\n", exitCount)
			os.Exit(0)
		}
	}
}

func main() {
	flag.StringVar(&url, "url", "", "URL to fetch, example: https://northflier4.streambox.com/light/light_status.php")
	flag.DurationVar(&expectedTimeDuration, "predicted", 10*time.Minute, "Expected time for fetching the URL based of previous observations")
	flag.DurationVar(&fetchDelay, "delay", 3*time.Second, "Delay between fetch attempts")
	flag.IntVar(&exitCount, "count", 5, "Number of successful fetches before program exit")
	flag.BoolVar(&forever, "forever", false, "Keep running indefinitely even after meeting success count")
	flag.Parse()

	if url == "" {
		flag.Usage()
		os.Exit(1)
	}

	startTime = time.Now()

	ticker := time.NewTicker(fetchDelay)
	defer ticker.Stop()

	for range ticker.C {
		fetch(url)
	}
}
