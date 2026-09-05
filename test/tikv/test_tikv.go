package main

import (
	"context"
	"log"
	"time"

	"github.com/tikv/client-go/v2/config"
	"github.com/tikv/client-go/v2/rawkv"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sec := config.Security{
		ClusterSSLCA:   "/nefsdata/meta/tikv-deploy/pd-12379/tls/ca.crt",
		ClusterSSLCert: "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.crt",
		ClusterSSLKey:  "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.pem",
	}

	pdAddrs := []string{
		"10.153.28.202:12379",
		"10.153.28.203:12379",
		"10.153.28.204:12379",
	}

	log.Println("Connecting to TiKV with TLS...")
	log.Printf("PD addresses: %v", pdAddrs)

	client, err := rawkv.NewClient(ctx, pdAddrs, sec)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	log.Println("Connected successfully!")

	testKey := []byte("test-tls-key")
	testValue := []byte("hello TLS")

	log.Println("Testing Put...")
	err = client.Put(ctx, testKey, testValue)
	if err != nil {
		log.Fatalf("Put failed: %v", err)
	}
	log.Println("Put succeeded!")

	log.Println("Testing Get...")
	val, err := client.Get(ctx, testKey)
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	log.Printf("Get succeeded! Value: %s", string(val))

	log.Println("Testing Delete...")
	err = client.Delete(ctx, testKey)
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	log.Println("Delete succeeded!")

	log.Println("All tests passed!")
}
