package hn

import (
	"fmt"
	"time"
)

type (
	Cache struct {
		client    *Client
		topItems  *topItemsList
		items     map[int]cacheItem
		timeLimit time.Duration
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

// TopItems retrieves the top item IDs from the cache.
func (c *Cache) TopItems() ([]int, error) {
	if !c.isCacheExpired() {
		return c.topItems.ids, nil
	}
	fmt.Println("cache top items expired, updating...")
	topItems, err := c.client.TopItems()
	c.topItems = &topItemsList{
		ids:       topItems,
		timestamp: time.Now(),
	}
	return c.topItems.ids, err
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
	if !c.isCacheItemExpired(id) {
		item := c.items[id]
		return item.item, nil
	}
	fmt.Println("cache item expired, updating...")
	item, err := c.client.GetItem(id)
	cacheItem := cacheItem{
		item:      item,
		timestamp: time.Now(),
	}
	c.items[id] = cacheItem
	return item, err
}

func (c *Cache) isCacheItemExpired(id int) bool {
	item, ok := c.items[id]
	if !ok {
		return true
	}
	return item.timestamp.Add(c.timeLimit).Before(time.Now())
}
