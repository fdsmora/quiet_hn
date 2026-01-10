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
	"sync"
	"time"

	"github.com/gophercises/quiet_hn/hn"
)

func main() {
	// parse flags
	var port, numStories int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	http.HandleFunc("/", handler(numStories, tpl))

	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handler(numStories int, tpl *template.Template) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		stories, err := getStories(numStories)
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

func getStories(numStories int) ([]item, error) {
	var client hn.Client
	ids, err := client.TopItems()
	if err != nil {
		return nil, err
	}
	var stories []item

	idsChan := make(chan idxID, numStories)
	cancelChan := make(chan struct{})
	storiesChan := make(chan result, numStories)
	continueChan := make(chan struct{})

	go sendIDs(idsChan, ids, cancelChan)

	wg := sync.WaitGroup{}

	go sendStories(idsChan, storiesChan, continueChan, cancelChan, &client, numStories, &wg)

	results := getResults(numStories, storiesChan, continueChan, cancelChan)
	wg.Wait()
	close(storiesChan)

	sort.Slice(results, func(i, j int) bool {
		return results[i].idx < results[j].idx
	})
	for _, r := range results {
		stories = append(stories, r.item)
	}
	return stories, nil
}

type idxID struct {
	idx, id int
}

type result struct {
	idx  int
	item item
	err  error
}

func sendIDs(idsChan chan<- idxID, ids []int, cancelChan <-chan struct{}) {
	defer close(idsChan)
	for i, id := range ids {
		select {
		case <-cancelChan:
			fmt.Println("cancelling id sender")
			return
		default:
			idsChan <- idxID{idx: i, id: id}
		}
	}
}

func sendStories(idsChan <-chan idxID,
	storiesChan chan<- result,
	continueChan <-chan struct{},
	cancelChan <-chan struct{},
	client *hn.Client,
	numStories int,
	wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	for {
		for range numStories {
			select {
			case <-cancelChan:
				fmt.Println("cancelling story fetcher")
				return
			default:
				wg.Go(func() {
					i, _ := <-idsChan
					hnItem, err := client.GetItem(i.id)
					if err != nil {
						storiesChan <- result{idx: i.idx, err: err}
					}
					//			fmt.Println("about to send story")
					storiesChan <- result{idx: i.idx, item: parseHNItem(hnItem)}
				})
			}
		}
		//fmt.Println("finished sending batch of 30 stories, waiting for continue")
		<-continueChan
		//fmt.Println("Continuing..")
	}
}

func getResults(numStories int, storiesChan <-chan result, continueChan chan struct{}, cancelChan chan struct{}) []result {
	var results []result
	for {
		for range numStories {
			story := <-storiesChan
			if story.err != nil {
				continue
			}
			if isStoryLink(story.item) {
				results = append(results, story)
			}
			if len(results) >= numStories {
				close(cancelChan)
				close(continueChan)
				return results
			}
		}
		//fmt.Printf("finished reading 30 times, total length: %d. Letting fetching func to continue\n", len(results))
		continueChan <- struct{}{}
		//fmt.Printf("Sent continue signal\n")
	}
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
