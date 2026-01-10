package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
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
		var client hn.Client
		ids, err := client.TopItems()
		if err != nil {
			http.Error(w, "Failed to load top stories", http.StatusInternalServerError)
			return
		}
		chStories := make(chan item)

		inChan := make(chan item, len(ids))
		outChan1 := make(chan item, len(ids))
		outChan2 := make(chan item, len(ids))
		hm := createMapOfChans(ids)

		abort := make(chan struct{})

		go sendItems(client, ids, inChan)
		go fanOutItems(inChan, outChan1, outChan2)
		go readSendStories(outChan2, hm, chStories)
		go sendStoriesWorkers(outChan1, abort, hm)
		stories := getStories(chStories, abort, numStories)

		<-abort

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

func createMapOfChans(ids []int) map[int]*itemChan {
	hm := make(map[int]*itemChan)
	for _, id := range ids {
		hm[id] = &itemChan{chItem: make(chan item, 1)}
	}
	return hm
}

// Fill the id channel with items that are stories only
func sendItems(client hn.Client, ids []int, inChan chan<- item) {
	for _, id := range ids {
		hnItem, err := client.GetItem(id)
		if err != nil {
			log.Fatalln("getting item:", err)
		}
		item := parseHNItem(hnItem)
		if isStoryLink(item) {
			inChan <- item
		}
	}
	close(inChan)
}

// Fan-out: duplicate ids from idChan to idChan1 and idChan2
func fanOutItems(inChan <-chan item, outChan1, outChan2 chan<- item) {
	for it := range inChan {
		outChan1 <- it
		outChan2 <- it
	}
	close(outChan1)
	close(outChan2)
}

func readSendStories(outChan2 <-chan item, hm map[int]*itemChan, chStories chan<- item) {
	for it := range outChan2 {
		//	fmt.Println("chan2 received id: ", it)
		it := <-hm[it.ID].chItem
		//	fmt.Println("received  id: ", it.ID)
		chStories <- it
	}
}

// Start a fixed number of worker goroutines
func sendStoriesWorkers(outChan1 <-chan item, abort chan struct{}, hm map[int]*itemChan) {
	const numWorkers = 30
	for range numWorkers {
		it, ok := <-outChan1
		// fmt.Println("chan1 received id: ", it)
		if !ok {
			// fmt.Println("chan1 not ok id: ", it)
			break
		}
		//	time.Sleep(time.Millisecond * time.Duration(i))
		go func() {
			for {
				select {
				case <-abort:
					fmt.Println("aborting...")
					return
				default:
					if !hm[it.ID].seen {
						// fmt.Println("sending id: ", it)
						hm[it.ID].chItem <- it
						hm[it.ID].seen = true
						close(hm[it.ID].chItem)
					}
					return
				}
			}
		}()
	}
}

func getStories(chStories chan item, abort chan struct{}, numStories int) []item {
	stories := []item{}
	for itm := range chStories {
		stories = append(stories, itm)
		if len(stories) >= numStories {
			close(abort)
			break
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

type itemChan struct {
	chItem chan item
	seen   bool
}

type templateData struct {
	Stories []item
	Time    time.Duration
}
