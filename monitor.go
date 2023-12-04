package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	filename = "websites.csv"
)

// init function runs before main.
func init() {
	if os.Getenv("ENV") == "development" {
		fmt.Println("Development environment enabled")
	}
}

type Website struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	DateAdded  string `json:"data_added"`
	Uptime     string `json:"uptime"`
	Interval   int    `json:"interval"`
}

// OpenFile opens a CSV file and updates a map.
func OpenFile(f string, sites map[string]int) {
	file, err := os.Open(f)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		seconds, err := strconv.Atoi(record[1])
		if err != nil {
			log.Fatal(err)
		}
		sites[record[0]] = seconds
	}

	return
}

// printTable displays a table of each website.
func printTable(website chan Website) {
	fmt.Println("\n-------------------------------------------------" +
		"\n| Status | Interval | URL                       |")
	for w := range website {
		fmt.Printf("-------------------------------------------------\n"+
			"| %-6s | %-8d | %-25s |\n", fmt.Sprint(w.StatusCode), w.Interval, w.URL)
	}
}

// runTask performs a task at a set interval. On error the task closes.
func runTask(wg *sync.WaitGroup, url string, site chan Website, seconds int) {
	defer wg.Done()
	errCh := make(chan error)
	ticker := time.NewTicker(time.Second * time.Duration(1))

	func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s, err := GetStatusCode(url)
				w := Website{StatusCode: s, Interval: seconds, URL: url}
				if err != nil {
					site <- w
					errCh <- fmt.Errorf("ticker error")
					return
				}
				site <- w
				ticker.Reset(time.Duration(seconds) * time.Second)
			case <-errCh:
				return
			}
		}
	}()

	close(errCh)
}

// GetStatusCode makes an HTTP request to get the status code of a website.
func GetStatusCode(url string) (int, error) {
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		os.Exit(1)
		return -1, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/116.0")

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("error, unexpected status code: %d\n", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

func main() {
	fmt.Println("Starting Downtime Monitor")
	websites := make(map[string]int)
	OpenFile(filename, websites)

	wg := new(sync.WaitGroup)
	site := make(chan Website)

	for k, v := range websites {
		wg.Add(1)
		go runTask(wg, k, site, v)
	}

	printTable(site)

	wg.Wait()
}
