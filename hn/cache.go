package hn

type Cache struct {
	client   *Client
	topItems []int
	items    map[int]Item
}

func NewCache(client *Client) *Cache {
	return &Cache{
		client:   client,
		topItems: nil,
		items:    make(map[int]Item),
	}
}

// TopItems retrieves the top item IDs from the cache.
func (c *Cache) TopItems() ([]int, error) {
	if c.topItems != nil {
		return c.topItems, nil
	}
	var err error
	c.topItems, err = c.client.TopItems()
	return c.topItems, err
}

// GetItem retrieves an item by its ID from the cache.
func (c *Cache) GetItem(id int) (Item, error) {
	if item, ok := c.items[id]; ok {
		return item, nil
	}
	var err error
	c.items[id], err = c.client.GetItem(id)
	return c.items[id], err
}
