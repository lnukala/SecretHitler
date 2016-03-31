package dnsimple

import (
	"fmt"
	"log"
	"strings"
	"zmq"

	DNS "github.com/rubyist/go-dnsimple"
)

//Domain :of the game
const Domain = "secrethitler.ml"

//GetClient :Get the API client for dnssimple
func GetClient() *DNS.DNSimpleClient {
	apiToken := "ncAbBihp7T2wHrqIrzBg1uhvWRfCUE6J"
	email := "leelakrishnanukala27@gmail.com"
	client := DNS.NewClient(apiToken, email)
	return client
}

//PrintDomains : List the domains under the concerned client
func PrintDomains(client *DNS.DNSimpleClient) {
	// Get a list of your domains (with error management)
	domains, error := client.Domains()
	if error != nil {
		log.Fatalln(error)
	}
	for _, domain := range domains {
		fmt.Printf("Domain: %s \n", domain.Name)
	}
}

//GetRecords :Get the A records associated with a domain
func GetRecords(client *DNS.DNSimpleClient) []DNS.Record {
	records, err := client.Records(Domain, "", "A")
	if err != nil {
		log.Fatal(err)
	}
	for _, record := range records {
		fmt.Printf("Record: %s -> %s\n", record.Name, record.Content)
	}
	print(len(records))
	return records
}

//AddRecord :Add an a record against a domain
func AddRecord(client *DNS.DNSimpleClient, recordname string) {
	ip := zmq.GetPublicIP()
	newRec := DNS.Record{Name: recordname, Content: ip, RecordType: "A"}
	client.CreateRecord(Domain, newRec)
}

//DeleteRecord :Delete the record entered for an IP
func DeleteRecord(client *DNS.DNSimpleClient, ip string) {
	records, err := client.Records(Domain, "", "A")
	if err != nil {
		log.Fatal(err)
	}
	for _, record := range records {
		if strings.Compare(ip, record.Content) == 0 {
			record.Delete(client)
			return
		}
	}
}
