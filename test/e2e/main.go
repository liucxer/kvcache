package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"kvcache/client"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("=== KVCache E2E Test Starting ===")
	startTime := time.Now()

	cfg := &client.Config{
		Node:       "146",
		TiKVPD:     "10.153.28.202:12379,10.153.28.203:12379,10.153.28.204:12379",
		CACert:     "/nefsdata/meta/tikv-deploy/pd-12379/tls/ca.crt",
		ClientCert: "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.crt",
		ClientKey:  "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.pem",
	}

	sdk, err := client.NewClient(cfg)
	if err != nil {
		log.Fatalf("FAIL: NewClient: %v", err)
	}
	defer sdk.Close()
	log.Printf("PASS: SDK initialized (node=146) [%v]", time.Since(startTime))

	time.Sleep(2 * time.Second)

	ctx := context.Background()
	testCount := 0
	passCount := 0

	runTest := func(name string, fn func() error) {
		testCount++
		start := time.Now()
		if err := fn(); err != nil {
			log.Printf("FAIL: %s: %v [%v]", name, err, time.Since(start))
		} else {
			passCount++
			log.Printf("PASS: %s [%v]", name, time.Since(start))
		}
	}

	// Test 1: Set + Get basic operation
	runTest("1. Set + Get", func() error {
		key := "e2e:test1:key1"
		val := []byte(fmt.Sprintf("value1-%d", time.Now().UnixNano()))

		if err := sdk.Set(ctx, key, val); err != nil {
			return fmt.Errorf("Set failed: %v", err)
		}
		got, err := sdk.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("Get failed: %v", err)
		}
		if string(got) != string(val) {
			return fmt.Errorf("value mismatch: got %q, want %q", got, val)
		}
		return nil
	})

	// Test 2: Delete operation
	runTest("2. Delete", func() error {
		key := "e2e:test2:todelete"
		val := []byte("to-be-deleted")

		if err := sdk.Set(ctx, key, val); err != nil {
			return fmt.Errorf("Set failed: %v", err)
		}
		if err := sdk.Delete(ctx, key); err != nil {
			return fmt.Errorf("Delete failed: %v", err)
		}
		_, err := sdk.Get(ctx, key)
		if err == nil {
			return fmt.Errorf("Get after Delete should have failed")
		}
		return nil
	})

	// Test 3: MSet + MGet batch operations
	runTest("3. MSet + MGet", func() error {
		kvs := map[string][]byte{
			"e2e:batch:a": []byte("batch-value-a"),
			"e2e:batch:b": []byte("batch-value-b"),
			"e2e:batch:c": []byte("batch-value-c"),
		}
		if err := sdk.MSet(ctx, kvs); err != nil {
			return fmt.Errorf("MSet failed: %v", err)
		}
		keys := []string{"e2e:batch:a", "e2e:batch:b", "e2e:batch:c"}
		got, err := sdk.MGet(ctx, keys)
		if err != nil {
			return fmt.Errorf("MGet failed: %v", err)
		}
		if len(got) != 3 {
			return fmt.Errorf("MGet returned %d keys, want 3", len(got))
		}
		for k, wantV := range kvs {
			if string(got[k]) != string(wantV) {
				return fmt.Errorf("MGet[%s] = %q, want %q", k, got[k], wantV)
			}
		}
		return nil
	})

	// Test 4: MDelete operation
	runTest("4. MDelete", func() error {
		kvs := map[string][]byte{
			"e2e:mdel:1": []byte("v1"),
			"e2e:mdel:2": []byte("v2"),
		}
		if err := sdk.MSet(ctx, kvs); err != nil {
			return fmt.Errorf("MSet failed: %v", err)
		}
		keys := []string{"e2e:mdel:1", "e2e:mdel:2"}
		if err := sdk.MDelete(ctx, keys); err != nil {
			return fmt.Errorf("MDelete failed: %v", err)
		}
		got, err := sdk.MGet(ctx, keys)
		if err != nil {
			return fmt.Errorf("MGet after MDelete failed: %v", err)
		}
		if len(got) != 0 {
			return fmt.Errorf("MGet after MDelete returned %d keys, want 0", len(got))
		}
		return nil
	})

	// Test 5: Multiple keys distribution
	runTest("5. Multiple keys", func() error {
		numKeys := 10
		kvs := make(map[string][]byte)
		for i := 0; i < numKeys; i++ {
			kvs[fmt.Sprintf("e2e:dist:%d", i)] = []byte(fmt.Sprintf("value-%d", i))
		}
		if err := sdk.MSet(ctx, kvs); err != nil {
			return fmt.Errorf("MSet failed: %v", err)
		}
		keys := make([]string, 0, numKeys)
		for k := range kvs {
			keys = append(keys, k)
		}
		got, err := sdk.MGet(ctx, keys)
		if err != nil {
			return fmt.Errorf("MGet failed: %v", err)
		}
		if len(got) != numKeys {
			return fmt.Errorf("MGet returned %d keys, want %d", len(got), numKeys)
		}
		return nil
	})

	// Test 6: Route cache
	runTest("6. Route cache", func() error {
		key := "e2e:cache:hot-key"
		val := []byte("cached-value")

		if err := sdk.Set(ctx, key, val); err != nil {
			return fmt.Errorf("Set failed: %v", err)
		}
		got1, err := sdk.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("Get1 failed: %v", err)
		}
		if string(got1) != string(val) {
			return fmt.Errorf("Get1 mismatch")
		}
		start := time.Now()
		got2, err := sdk.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("Get2 failed: %v", err)
		}
		elapsed := time.Since(start)
		if string(got2) != string(val) {
			return fmt.Errorf("Get2 mismatch")
		}
		if elapsed > 50*time.Millisecond {
			log.Printf("  WARN: second Get took %v (expected faster with cache)", elapsed)
		}
		return nil
	})

	// Test 7: Get non-existent key
	runTest("7. Get non-existent key", func() error {
		_, err := sdk.Get(ctx, "e2e:does-not-exist:xyz123")
		if err == nil {
			return fmt.Errorf("Get non-existent key should have failed")
		}
		return nil
	})

	// Test 8: Large value (>1MB triggers disk storage)
	runTest("8. Large value (1MB+)", func() error {
		key := "e2e:large:1mb"
		val := make([]byte, 1024*1024+100)
		for i := range val {
			val[i] = byte(i % 256)
		}
		if err := sdk.Set(ctx, key, val); err != nil {
			return fmt.Errorf("Set large value failed: %v", err)
		}
		got, err := sdk.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("Get large value failed: %v", err)
		}
		if len(got) != len(val) {
			return fmt.Errorf("size mismatch: got %d, want %d", len(got), len(val))
		}
		for i := 0; i < 100; i++ {
			if got[i] != val[i] {
				return fmt.Errorf("byte mismatch at index %d", i)
			}
		}
		return sdk.Delete(ctx, key)
	})

	// Summary
	fmt.Printf("\n========================================\n")
	fmt.Printf("E2E Test Results: %d/%d passed\n", passCount, testCount)
	fmt.Printf("Duration: %v\n", time.Since(startTime))
	if passCount == testCount {
		fmt.Println("ALL TESTS PASSED ✓")
	} else {
		fmt.Printf("FAILED: %d/%d\n", testCount-passCount, testCount)
	}
	fmt.Printf("========================================\n")
}
