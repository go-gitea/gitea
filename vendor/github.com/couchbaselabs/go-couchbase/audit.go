package couchbase

import ()

// Sample data:
// {"disabled":["12333", "22244"],"uid":"132492431","auditdEnabled":true,
//  "disabledUsers":[{"name":"bill","domain":"local"},{"name":"bob","domain":"local"}],
//  "logPath":"/Users/johanlarson/Library/Application Support/Couchbase/var/lib/couchbase/logs",
//  "rotateInterval":86400,"rotateSize":20971520}
type AuditSpec struct {
	Disabled       []uint32    `json:"disabled"`
	Uid            string      `json:"uid"`
	AuditdEnabled  bool        `json:"auditdEnabled`
	DisabledUsers  []AuditUser `json:"disabledUsers"`
	LogPath        string      `json:"logPath"`
	RotateInterval int64       `json:"rotateInterval"`
	RotateSize     int64       `json:"rotateSize"`
}

type AuditUser struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

func (c *Client) GetAuditSpec() (*AuditSpec, error) {
	ret := &AuditSpec{}
	err := c.parseURLResponse("/settings/audit", ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
