package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/brandon-wallace/downtime-monitor.git/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

const port = ":8000"

var websiteIDSet = make(map[int]struct{})

// init function runs before main.
func init() {
	if os.Getenv("ENV") == "development" {
		fmt.Println("Development environment enabled")
	}
}

type Site interface {
	AddSite(website *models.Website) (int, error)
	GetAllSites() ([]*models.Website, error)
}

type siteDB struct {
	db *sql.DB
}

// AddSite adds a website to the database.
func (site *siteDB) AddSite(w *models.Website) (int, error) {
	statement, err := site.db.Prepare("INSERT INTO websites (name, url, status_code, date_added, uptime, interval) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("error preparing statement %v", err)
	}
	result, err := statement.Exec(w.Name, w.URL, w.StatusCode, w.DateAdded, w.Uptime, w.Interval)
	if err != nil {
		return 0, fmt.Errorf("error executing statement %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error retreiving id %v %v", id, err)
	}

	return int(id), err
}

// GetAllSites queries the database for all websites.
func (site *siteDB) GetAllSites() ([]*models.Website, error) {
	rows, err := site.db.Query("SELECT id, name, url, status_code, date_added, uptime, interval FROM websites ORDER BY id ASC")
	if err != nil {
		fmt.Println("1 ERROR ->", err)
		return nil, err
	}
	defer rows.Close()

	websites := []*models.Website{}
	for rows.Next() {
		w := &models.Website{}
		rows.Scan()
		if err := rows.Scan(&w.ID, &w.Name, &w.URL, &w.StatusCode, &w.DateAdded, &w.Uptime, &w.Interval); err != nil {
			fmt.Println("2 ERROR ->", err)
			return nil, err
		}
		websites = append(websites, w)
	}
	if err = rows.Err(); err != nil {
		fmt.Println("3 ERROR ->", err)
		return nil, err
	}

	return websites, nil
}

var site Site

func InitSite(s Site) {
	site = s
}

type templateData struct {
	Website  models.Website
	Websites []*models.Website
}

// formatTimedelta converts and formats seconds into days, hours, and minutes.
func formatTimedelta(t time.Duration) string {
	days := int(t.Seconds() / 86400)
	hours := int(t.Seconds()/3600) % 24
	minutes := int(t.Seconds()/60) % 60
	if days == 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}

// runTask performs a task at a set interval. On error the task closes.
func runTask(wg *sync.WaitGroup, url string, site chan models.Website, seconds int) {
	defer wg.Done()
	errCh := make(chan error)
	ticker := time.NewTicker(time.Second * time.Duration(1))

	func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s, err := GetStatusCode(url)
				w := models.Website{StatusCode: s, Interval: seconds, URL: url}
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
		return resp.StatusCode, fmt.Errorf("error, unexpected status code %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

// Index is the landing page for the application.
func Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// POST
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			fmt.Println("Form parse error", err)
			return
		}

		status := 200
		log.Printf("Status Code is %d for %s", status, r.PostFormValue("url"))

		now := time.Now()
		future := now.AddDate(0, 0, 4)
		future = future.Add(time.Minute * 123)
		timedelta := future.Sub(now)
		i, _ := strconv.ParseInt(r.PostFormValue("interval"), 0, 0)
		w := models.Website{
			Name:       r.PostFormValue("name"),
			URL:        r.PostFormValue("url"),
			StatusCode: status,
			DateAdded:  now.Format("2006-01-02 15:03:04"),
			Uptime:     formatTimedelta(timedelta),
			Interval:   int(i),
		}

		fmt.Println(r.PostFormValue("name"), r.PostFormValue("url"), status, now.Format("2006-01-02 15:03:04"), formatTimedelta(timedelta), int(i))
		id, err := site.AddSite(&w)
		if err != nil {
			log.Printf("insert error: %v", err)
		}
		_, exists := websiteIDSet[int(id)]
		if !exists {
			websiteIDSet[int(id)] = struct{}{}
		}
	}

	// GET
	websites, err := site.GetAllSites()
	if err != nil {
		fmt.Println(err)
	}

	siteCh := make(chan models.Website)
	go func() {
		var wg sync.WaitGroup
		for i, w := range websites {
			_, exists := websiteIDSet[websites[i].ID]
			if !exists {
				websiteIDSet[websites[i].ID] = struct{}{}
				wg.Add(1)
				go runTask(&wg, w.URL, siteCh, w.Interval)
				fmt.Println(w.Name)
			}
		}
		wg.Wait()

	}()
	for s := range siteCh {
		fmt.Println(s)
	}

	for k, _ := range websiteIDSet {
		fmt.Println("key ID:", k)
	}

	files := []string{
		"./web/html/layout.html",
		"./web/html/pages/index.html",
	}
	templates, err := template.ParseFiles(files...)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	data := &templateData{
		Websites: websites,
	}

	if err := templates.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "Internal Server Error", 500)
	}
}

// Delete deletes one website from the database
func Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		fmt.Println("Error converting to integer", err)
	}

	db, err := models.OpenDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := models.Delete(db, id); err != nil {
		fmt.Println("Delete error", err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// main
func main() {
	fmt.Println("Starting Downtime Monitor")

	db, err := models.OpenDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	InitSite(&siteDB{db: db})

	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("./web/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// mux.HandleFunc("/echo", echo)
	mux.HandleFunc("/", Index)
	mux.HandleFunc("/site/", Delete)

	logErr := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	server := &http.Server{
		Addr:     port,
		ErrorLog: logErr,
		Handler:  mux,
	}

	fmt.Printf("Starting server http://127.0.0.1 on %s\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
