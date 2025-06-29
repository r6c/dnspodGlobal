package main

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/libdns/libdns"
	dnspod "github.com/r6c/dnspodGlobal"
)

func main() {
	token := os.Getenv("DNSPOD_TOKEN")
	if token == "" {
		fmt.Printf("DNSPOD_TOKEN not set\n")
		return
	}
	zone := os.Getenv("ZONE")
	if zone == "" {
		fmt.Printf("ZONE not set\n")
		return
	}

	// Initialize provider with correct field name
	provider := dnspod.Provider{LoginToken: token}

	// Get existing records
	records, err := provider.GetRecords(context.TODO(), zone)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return
	}

	testName := "libdns-test"
	var existingRecord libdns.Record

	fmt.Printf("Existing records:\n")
	for _, record := range records {
		rr := record.RR()
		fmt.Printf("%s (.%s): %s, %s\n", rr.Name, zone, rr.Data, rr.Type)
		if rr.Name == testName+"."+zone+"." && rr.Type == "TXT" {
			existingRecord = record
		}
	}

	if existingRecord != nil {
		// Update existing record using SetRecords
		fmt.Printf("Updating existing entry for %s\n", testName)
		_, err = provider.SetRecords(context.TODO(), zone, []libdns.Record{
			libdns.TXT{
				Name: testName + "." + zone + ".",
				Text: fmt.Sprintf("Updated test entry created by libdns %s", time.Now().Format(time.RFC3339)),
				TTL:  time.Duration(600) * time.Second,
			},
		})
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
		} else {
			fmt.Printf("Record updated successfully\n")
		}
	} else {
		// Create new record
		fmt.Printf("Creating new entry for %s\n", testName)
		_, err = provider.AppendRecords(context.TODO(), zone, []libdns.Record{
			libdns.TXT{
				Name: testName + "." + zone + ".",
				Text: fmt.Sprintf("This is a test entry created by libdns %s", time.Now().Format(time.RFC3339)),
				TTL:  time.Duration(600) * time.Second,
			},
		})
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
		} else {
			fmt.Printf("Record created successfully\n")
		}
	}

	// Example of creating an A record using Address type
	fmt.Printf("\nCreating A record example\n")
	ip, _ := netip.ParseAddr("192.0.2.1") // RFC5737 documentation IP
	_, err = provider.AppendRecords(context.TODO(), zone, []libdns.Record{
		libdns.Address{
			Name: "test-a." + zone + ".",
			IP:   ip,
			TTL:  time.Duration(300) * time.Second,
		},
	})
	if err != nil {
		fmt.Printf("ERROR creating A record: %s\n", err.Error())
	} else {
		fmt.Printf("A record created successfully\n")
	}
}
