package couchbase

/*

The goal here is to map a hostname:port combination to another hostname:port
combination. The original hostname:port gives the name and regular KV port
of a couchbase server. We want to determine the corresponding SSL KV port.

To do this, we have a pool services structure, as obtained from
the /pools/default/nodeServices API.

For a fully configured two-node system, the structure may look like this:
{"rev":32,"nodesExt":[
	{"services":{"mgmt":8091,"mgmtSSL":18091,"fts":8094,"ftsSSL":18094,"indexAdmin":9100,"indexScan":9101,"indexHttp":9102,"indexStreamInit":9103,"indexStreamCatchup":9104,"indexStreamMaint":9105,"indexHttps":19102,"capiSSL":18092,"capi":8092,"kvSSL":11207,"projector":9999,"kv":11210,"moxi":11211},"hostname":"172.23.123.101"},
	{"services":{"mgmt":8091,"mgmtSSL":18091,"indexAdmin":9100,"indexScan":9101,"indexHttp":9102,"indexStreamInit":9103,"indexStreamCatchup":9104,"indexStreamMaint":9105,"indexHttps":19102,"capiSSL":18092,"capi":8092,"kvSSL":11207,"projector":9999,"kv":11210,"moxi":11211,"n1ql":8093,"n1qlSSL":18093},"thisNode":true,"hostname":"172.23.123.102"}]}

In this case, note the "hostname" fields, and the "kv" and "kvSSL" fields.

For a single-node system, perhaps brought up for testing, the structure may look like this:
{"rev":66,"nodesExt":[
	{"services":{"mgmt":8091,"mgmtSSL":18091,"indexAdmin":9100,"indexScan":9101,"indexHttp":9102,"indexStreamInit":9103,"indexStreamCatchup":9104,"indexStreamMaint":9105,"indexHttps":19102,"kv":11210,"kvSSL":11207,"capi":8092,"capiSSL":18092,"projector":9999,"n1ql":8093,"n1qlSSL":18093},"thisNode":true}],"clusterCapabilitiesVer":[1,0],"clusterCapabilities":{"n1ql":["enhancedPreparedStatements"]}}

Here, note that there is only a single entry in the "nodeExt" array and that it does not have a "hostname" field.
We will assume that either hostname fields are present, or there is only a single node.
*/

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func ParsePoolServices(jsonInput string) (*PoolServices, error) {
	ps := &PoolServices{}
	err := json.Unmarshal([]byte(jsonInput), ps)
	return ps, err
}

func MapKVtoSSL(hostport string, ps *PoolServices) (string, error) {
	colonIndex := strings.LastIndex(hostport, ":")
	if colonIndex < 0 {
		return "", fmt.Errorf("Unable to find host/port separator in %s", hostport)
	}
	host := hostport[0:colonIndex]
	port := hostport[colonIndex+1:]
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("Unable to parse host/port combination %s: %v", hostport, err)
	}

	var ns *NodeServices
	if len(ps.NodesExt) == 1 {
		ns = &(ps.NodesExt[0])
	} else {
		for i := range ps.NodesExt {
			hostname := ps.NodesExt[i].Hostname
			if len(hostname) == 0 {
				// in case of missing hostname, check for 127.0.0.1
				hostname = "127.0.0.1"
			}
			if hostname == host {
				ns = &(ps.NodesExt[i])
				break
			}
		}
	}

	if ns == nil {
		return "", fmt.Errorf("Unable to parse host/port combination %s: no matching node found among %d", hostport, len(ps.NodesExt))
	}
	kv, found := ns.Services["kv"]
	if !found {
		return "", fmt.Errorf("Unable to map host/port combination %s: target host has no kv port listed", hostport)
	}
	kvSSL, found := ns.Services["kvSSL"]
	if !found {
		return "", fmt.Errorf("Unable to map host/port combination %s: target host has no kvSSL port listed", hostport)
	}
	if portInt != kv {
		return "", fmt.Errorf("Unable to map hostport combination %s: expected port %d but found %d", hostport, portInt, kv)
	}
	return fmt.Sprintf("%s:%d", host, kvSSL), nil
}
