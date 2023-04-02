package main

import (
	"crypto/tls" // package for creating a secure HTTP client
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
	"golang.org/x/net/html"
)

func main() {
	r := mux.NewRouter()

	// Serve static files from the "static" directory
	staticDir := "/css/"
	staticHandler := http.StripPrefix(staticDir, http.FileServer(http.Dir("css")))
	r.PathPrefix(staticDir).Handler(staticHandler)

	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/crawl", crawlHandler).Methods("POST")

	log.Println("Listening on :8080...")
	http.ListenAndServe(":8080", r)
}

// Handler for the home page that displays a form for entering a URL
func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("index.html"))
	tmpl.Execute(w, nil)
}

func crawlHandler(w http.ResponseWriter, r *http.Request) {
	// Get the extensions from the user and split them into a slice
	extensions := strings.Split(r.FormValue("extensions"), ",")
	urlStr := r.FormValue("url")

	// Parse the URL to get its scheme, host, and port
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Println(err)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Create a secure HTTP client that will ignore invalid certificates (e.g. self-signed) and allow us to crawl HTTPS sites
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	visited := make(map[string]bool)
	toVisit := []string{urlStr}
	links := []string{}

	for len(toVisit) > 0 {
		urlStr := toVisit[0]
		toVisit = toVisit[1:]

		if visited[urlStr] {
			continue
		}

		visited[urlStr] = true

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			log.Println(err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
			continue
		}

		defer resp.Body.Close()

		z := html.NewTokenizer(resp.Body)

		for {
			tt := z.Next()

			switch tt {
			case html.ErrorToken:
				// Filter the links based on the allowed extensions
				filteredLinks := filterLinks(links, extensions)

				tmpl := template.Must(template.ParseFiles("results.html"))
				tmpl.Execute(w, filteredLinks)
				return
			case html.StartTagToken, html.SelfClosingTagToken:
				t := z.Token()

				if t.Data == "a" {
					for _, attr := range t.Attr {
						if attr.Key == "href" {
							linkURL, err := u.Parse(attr.Val)
							if err != nil {
								continue
							}

							// Only visit links that use the same scheme as the original URL
							if linkURL.Scheme != u.Scheme {
								continue
							}

							// Add the link to the list of links to visit
							toVisit = append(toVisit, linkURL.String())
							links = append(links, linkURL.String())
							break
						}
					}
				}
			}
		}
	}
}

// filterLinks filters the input links based on the allowed extensions
func filterLinks(links []string, extensions []string) []string {
	filteredLinks := []string{}

	for _, link := range links {
		ext := filepath.Ext(link)

		for _, allowedExt := range extensions {
			if strings.EqualFold(ext, allowedExt) {
				filteredLinks = append(filteredLinks, link)
				break
			}
		}
	}

	return filteredLinks
}
