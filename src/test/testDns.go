package main

import (
	"dnsimple"
	"fmt"
	"strconv"
	"time"
)

func main() {
	client := dnsimple.GetClient()

	fmt.Println("Step0: The domain we are using:")
	dnsimple.PrintDomains(client)

	fmt.Println("Step1: Current records:")
	records := dnsimple.GetRecords(client)

	if len(records) == 0 {
		fmt.Println("No record exists, adding new one...")
		dnsimple.AddRecord(client, "testrecord")
	}

	for {
		if rec := dnsimple.GetRecords(client); len(rec) == 0 {
			time.Sleep(30 * 1000 * time.Millisecond)
			fmt.Println("30 seconds: no record has been added")
		} else {
			fmt.Println("Some records exist")
			break
		}
	}

	fmt.Println("Step2: Deleting all the records...")
	for _, record := range records {
		record.Delete(client)
	}

	for {
		if rec := dnsimple.GetRecords(client); len(rec) != 0 {
			time.Sleep(10 * 1000 * time.Millisecond)
			fmt.Println("10 seconds: " + strconv.Itoa(len(rec)) + " remain")
		} else {
			fmt.Println("Done delete")
			break
		}
	}
}
