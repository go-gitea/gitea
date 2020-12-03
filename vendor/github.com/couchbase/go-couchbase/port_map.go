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
	"net"
	"strconv"
)

func ParsePoolServices(jsonInput string) (*PoolServices, error) {
	ps := &PoolServices{}
	err := json.Unmarshal([]byte(jsonInput), ps)
	return ps, err
}

// Accepts a "host:port" string representing the KV TCP port and the pools
// nodeServices payload and returns a host:port string representing the KV
// TLS port on the same node as the KV TCP port.
// Returns the original host:port if in case of local communication (services
// on the same node as source)
func MapKVtoSSL(hostport string, ps *PoolServices) (string, bool, error) {
	return MapKVtoSSLExt(hostport, ps, false)
}

func MapKVtoSSLExt(hostport string, ps *PoolServices, force bool) (string, bool, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", false, fmt.Errorf("Unable to split hostport %s: %v", hostport, err)
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", false, fmt.Errorf("Unable to parse host/port combination %s: %v", hostport, err)
	}

	var ns *NodeServices
	for i := range ps.NodesExt {
		hostname := ps.NodesExt[i].Hostname
		if len(hostname) != 0 && hostname != host {
			/* If the hostname is the empty string, it means the node (and by extension
			   the cluster) is configured on the loopback. Further, it means that the client
			   should use whatever hostname it used to get the nodeServices information in
			   the first place to access the cluster. Thus, when the hostname is empty in
			   the nodeService entry we can assume that client will use the hostname it used
			   to access the KV TCP endpoint - and thus that it automatically "matches".
			   If hostname is not empty and doesn't match then we move to the next entry.
			*/
			continue
		}
		kvPort, found := ps.NodesExt[i].Services["kv"]
		if !found {
			/* not a node with a KV service  */
			continue
		}
		if kvPort == portInt {
			ns = &(ps.NodesExt[i])
			break
		}
	}

	if ns == nil {
		return "", false, fmt.Errorf("Unable to parse host/port combination %s: no matching node found among %d", hostport, len(ps.NodesExt))
	}
	kvSSL, found := ns.Services["kvSSL"]
	if !found {
		return "", false, fmt.Errorf("Unable to map host/port combination %s: target host has no kvSSL port listed", hostport)
	}

	//Don't encrypt for communication between local nodes
	if !force && (len(ns.Hostname) == 0 || ns.ThisNode) {
		return hostport, false, nil
	}

	ip := net.ParseIP(host)
	if ip != nil && ip.To4() == nil && ip.To16() != nil { // IPv6 and not a FQDN
		// Prefix and suffix square brackets as SplitHostPort removes them,
		// see: https://golang.org/pkg/net/#SplitHostPort
		host = "[" + host + "]"
	}

	return fmt.Sprintf("%s:%d", host, kvSSL), true, nil
}
