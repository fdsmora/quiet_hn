package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/fdsmora/gophercises/quiet_hn/hn"
)

type (
	HN interface {
		TopItems() ([]int, error)
		GetItem(id int) (hn.Item, error)
	}
	handler struct {
		client HN
	}
)

func newHandler(client HN) *handler {
	return &handler{client: client}
}

func main() {
	// parse flags
	var port, numStories, cacheDuration int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.IntVar(&cacheDuration, "cache_duration", 10, "the cache duration in seconds")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	handler := newHandler(hn.NewCache(&hn.Client{}, time.Duration(cacheDuration)*time.Second))
	http.HandleFunc("/", handler.getHandler(numStories, tpl))

	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func (h *handler) getHandler(numStories int, tpl *template.Template) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		stories, err := h.getTopStories(numStories)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := templateData{
			Stories: stories,
			Time:    time.Since(start),
		}
		err = tpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to process the template", http.StatusInternalServerError)
			return
		}
	})
}

func (h *handler) getTopStories(numStories int) ([]item, error) {
	ids, err := h.client.TopItems()
	if err != nil {
		return nil, err
	}
	var stories []item
	var at int
	for len(stories) < numStories {
		need := (numStories - len(stories)) + 5/4
		stories = append(stories, h.getStories(ids[at:at+need])...)
		at += need
	}
	return stories[:numStories], nil
}

func (h *handler) getStories(ids []int) []item {
	type result struct {
		idx  int
		item item
		err  error
	}
	resultsChan := make(chan result)
	for i := 0; i < len(ids); i++ {
		go func(idx, id int) {
			hnItem, err := h.client.GetItem(id)
			if err != nil {
				resultsChan <- result{idx: idx, err: err}
			}
			resultsChan <- result{idx: idx, item: parseHNItem(hnItem)}
		}(i, ids[i])
	}
	var results []result
	for i := 0; i < len(ids); i++ {
		results = append(results, <-resultsChan)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].idx < results[j].idx
	})
	var stories []item
	for _, r := range results {
		if r.err != nil {
			continue
		}
		if isStoryLink(r.item) {
			stories = append(stories, r.item)
		}
	}
	return stories
}

func isStoryLink(item item) bool {
	return item.Type == "story" && item.URL != ""
}

func parseHNItem(hnItem hn.Item) item {
	ret := item{Item: hnItem}
	url, err := url.Parse(ret.URL)
	if err == nil {
		ret.Host = strings.TrimPrefix(url.Hostname(), "www.")
	}
	return ret
}

// item is the same as the hn.Item, but adds the Host field
type item struct {
	hn.Item
	Host string
}

type templateData struct {
	Stories []item
	Time    time.Duration
}
