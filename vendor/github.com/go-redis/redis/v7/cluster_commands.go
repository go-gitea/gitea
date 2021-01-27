package redis

import "sync/atomic"

func (c *ClusterClient) DBSize() *IntCmd {
	cmd := NewIntCmd("dbsize")
	var size int64
	err := c.ForEachMaster(func(master *Client) error {
		n, err := master.DBSize().Result()
		if err != nil {
			return err
		}
		atomic.AddInt64(&size, n)
		return nil
	})
	if err != nil {
		cmd.SetErr(err)
		return cmd
	}
	cmd.val = size
	return cmd
}
