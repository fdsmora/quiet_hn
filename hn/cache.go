package hn

import (
	"fmt"
	"sync"
	"time"
)

type (
	Cache struct {
		client         *Client
		topItems       *topItemsList
		items          map[int]cacheItem
		timeLimit      time.Duration
		updaterStarted bool
		mutex          sync.Mutex
	}
	topItemsList struct {
		ids       []int
		timestamp time.Time
	}
	cacheItem struct {
		item      Item
		timestamp time.Time
	}
)

func NewCache(client *Client, timeLimit time.Duration) *Cache {
	return &Cache{
		client:    client,
		topItems:  nil,
		items:     make(map[int]cacheItem),
		timeLimit: timeLimit,
	}
}

func (c *Cache) startBackgroundUpdater() {
	go func() {
		for {
			c.updateTopItems()
			c.updateItems()
			ticker := time.NewTicker(c.timeLimit * time.Second)
			<-ticker.C
		}
	}()
}

// TopItems retrieves the top item IDs from the cache.
func (c *Cache) TopItems() ([]int, error) {
	if c.updaterStarted == false {
		c.startBackgroundUpdater()
		c.updaterStarted = true
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.topItems != nil {
		return c.topItems.ids, nil
	}
	/* 	if !c.isCacheExpired() {
		return c.topItems.ids, nil
	} */
	err := c.updateTopItems()
	return c.topItems.ids, err
}

func (c *Cache) updateTopItems() error {
	fmt.Println("cache top items expired, updating...")
	topItems, err := c.client.TopItems()
	if err != nil {
		return fmt.Errorf("updating top items: %w", err)
	}
	c.topItems = &topItemsList{
		ids:       topItems,
		timestamp: time.Now(),
	}
	return nil
}

// isCacheExpired checks if the cache is expired.
func (c *Cache) isCacheExpired() bool {
	if c.topItems == nil {
		return true // Cache is empty, so it's considered expired.
	}
	return c.topItems.timestamp.Add(c.timeLimit).Before(time.Now())
}

// GetItem retrieves an item by its ID from the cache.
func (c *Cache) GetItem(id int) (Item, error) {
	if item, ok := c.items[id]; ok {
		return item.item, nil
	}
	/* 	if !c.isCacheItemExpired(id) {
		item := c.items[id]
		return item.item, nil
	} */
	fmt.Println("cache item expired, updating...")
	err := c.updateItem(id)
	return c.items[id].item, err
}

func (c *Cache) updateItems() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for id, _ := range c.items {
		err := c.updateItem(id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) updateItem(id int) error {
	fmt.Println("updating cache item ", id)
	newItem, err := c.client.GetItem(id)
	if err != nil {
		return fmt.Errorf("updating cache item %d: %w", id, err)
	}
	c.items[id] = cacheItem{
		item:      newItem,
		timestamp: time.Now(),
	}
	return nil
}

func (c *Cache) isCacheItemExpired(id int) bool {
	item, ok := c.items[id]
	if !ok {
		return true
	}
	return item.timestamp.Add(c.timeLimit).Before(time.Now())
}
