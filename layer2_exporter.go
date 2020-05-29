/*

   Purpose:  This is a layer2 prometheus arp exporter.

   Author:   Matthew Rogers
   Date:     2020-05-25
   License:  GPLv2

   Revision: v1.0.0

*/

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/j-keck/arping"
	"github.com/klauspost/oui"
)

//Globals
var completeScan = make(map[string]string)
var targetSubnet = "UNKNOWN"
var ouiURL = "http://standards-oui.ieee.org/oui.txt"

//scan should be running constantly.... once done the metrics table should be updated.
func showMetrics(w http.ResponseWriter, req *http.Request) {

	//build oui
	db, err := oui.OpenStaticFile("oui.txt")
	if err != nil {
		fmt.Println(err)
	}

	//target CIDR
	fmt.Fprintf(w, "layer2targetCidr{targetCidr=\"%s\"} 1\n", targetSubnet)
	//host count
	fmt.Fprintln(w, "layer2hostCount", len(completeScan))

	//{mac} ipaddress
	for key, value := range completeScan {
		
		var ouiOut = ""

		//lookup the OUI on render
		entry, err := db.Query(value)
		if err == oui.ErrNotFound {
			ouiOut = "UNKNOWN"
		} else if err != nil {
			panic(err)
		} else {
			ouiOut = entry.Manufacturer
		}


		fmt.Fprintf(w, "layer2host{ip=\"%s\", mac=\"%s\", oui=\"%s\"} 1\n", key,value,ouiOut)
	}
}

func main() {
	//Let's create our maps!
	
	workingScan := make(map[string]string)

	//take arguments
	flag.Parse()
	scanCidr := flag.Arg(0)
	targetSubnet = scanCidr
	listenAddress := ":9095"
	
	//custom port and interface
	if len(flag.Arg(1)) > 0 {
		listenAddress = flag.Arg(1)
	}


	http.HandleFunc("/metrics", showMetrics)
	
	//Server is parallel
	go http.ListenAndServe(listenAddress, nil)
	fmt.Println("Server is up....",listenAddress)

	
	//check for bad CIDR
	_, ipv4Net, err := net.ParseCIDR(scanCidr)
	if err != nil {
		fmt.Println("Provide a valid CIDR Subnet as an argument")
		fmt.Println("Example: go_network_scanner 192.168.0.1/24")
		log.Fatal(err)
	}

	fmt.Println("Net:", ipv4Net)
	startAddress, endAddress := cidr.AddressRange(ipv4Net)
	fmt.Println("Start/End:", startAddress, endAddress)
	fmt.Println("Count:", cidr.AddressCount(ipv4Net))

	//Scan Loop
	//	scan INTO working scan which starts with zero values
	//	once scan done, move working into finished, and zero out working
	//	restart scan
	for {
		startTime := time.Now()
		scanSubnet(startAddress, endAddress, workingScan)

		//scan finished
		println("*SCAN DONE*")
		for key, value := range workingScan {
			println("key:",key,"value:",value)
		}

		//wipe the prior scan
		for key := range completeScan {
			delete(completeScan, key)
		}

		//copy to it
		for key, value := range workingScan {
			completeScan[key] = value
		}

		//wipe working scan
		for key := range workingScan {
			delete(workingScan, key)
		}
		finishedTime := time.Now()
		fmt.Println("Total Scan Time:", finishedTime.Sub(startTime))
		
		//put a pause in scanning cycles
		time.Sleep(1 * time.Second)
	}

}

//this is the scan function
func scanSubnet(startAddress net.IP, endAddress net.IP, workingScan map[string]string) {

	//Go Through IP Address Range
	for ipAddress := startAddress; bytes.Compare(ipAddress, cidr.Inc(endAddress)) != 0; ipAddress = cidr.Inc(ipAddress) {

		arping.SetTimeout(100 * time.Millisecond)

		macAddress, _, _ := arping.Ping(ipAddress)

		if len(macAddress) > 0 {
			fmt.Println("SCAN:", ipAddress, macAddress)
			workingScan[ipAddress.String()] = macAddress.String()
		}

	}

}
