package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/tikv/client-go/v2/config"
	"github.com/tikv/client-go/v2/rawkv"
)

func main() {
	pdAddrs := []string{"10.153.28.202:12379", "10.153.28.203:12379", "10.153.28.204:12379"}
	caPath := "/nefsdata/meta/tikv-deploy/pd-12379/tls/ca.crt"
	certPath := "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.crt"
	keyPath := "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.pem"

	sec := config.Security{
		ClusterSSLCA:   caPath,
		ClusterSSLCert: certPath,
		ClusterSSLKey:  keyPath,
	}

	ctx := context.Background()
	cli, err := rawkv.NewClient(ctx, pdAddrs, sec)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer cli.Close()

	// Scan instances
	instPrefix := []byte("/kvcache/instances/")
	instEnd := []byte("/kvcache/instances0")
	instKeys, instVals, err := cli.Scan(ctx, instPrefix, instEnd, 1000)
	if err != nil {
		log.Fatalf("scan instances: %v", err)
	}
	fmt.Println("=== Active instances ===")
	for i, k := range instKeys {
		var info struct {
			Name      string `json:"name"`
			Node      string `json:"node"`
			Addr      string `json:"addr"`
			RawAddr   string `json:"raw_addr"`
			Capacity  int64  `json:"capacity"`
			Available int64  `json:"available"`
		}
		json.Unmarshal(instVals[i], &info)
		fmt.Printf("  %s -> node=%s addr=%s raw=%s\n", string(k), info.Node, info.Addr, info.RawAddr)
	}

	// Scan index keys (sample up to 100)
	idxPrefix := []byte("/kvcache/index/")
	idxEnd := []byte("/kvcache/index0")
	idxKeys, idxVals, err := cli.Scan(ctx, idxPrefix, idxEnd, 100)
	if err != nil {
		log.Fatalf("scan index: %v", err)
	}
	fmt.Printf("\n=== Index entries (sample %d, total >?) ===\n", len(idxKeys))
	prefixCount := make(map[string]int)
	for i, k := range idxKeys {
		keyStr := strings.TrimPrefix(string(k), "/kvcache/index/")
		var data struct {
			Instance string `json:"instance"`
		}
		json.Unmarshal(idxVals[i], &data)
		// Extract prefix (before first :)
		parts := strings.SplitN(keyStr, ":", 2)
		pref := parts[0]
		prefixCount[pref]++
		if i < 10 {
			fmt.Printf("  key=%s -> instance=%s\n", keyStr, data.Instance)
		}
	}
	fmt.Println("\n=== Key prefix summary ===")
	for p, c := range prefixCount {
		fmt.Printf("  prefix=%q count=%d\n", p, c)
	}

	// Count total index entries
	allKeys, _, err := cli.Scan(ctx, idxPrefix, idxEnd, 100000)
	if err == nil {
		fmt.Printf("\nTotal index entries: %d\n", len(allKeys))
	}
}