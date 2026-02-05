// //nolint
package main

import (
	"fmt"
	"time"

	"github.com/theojulienne/go-wireless"
)

func main() {
	ifaces := wireless.Interfaces()
	fmt.Printf("Interfaces:\n%+v\n", ifaces)

	iface, ok := wireless.DefaultInterface()
	if !ok {
		panic("no wifi interfaces found")
	}

	fmt.Printf("\nDefault interface:\n%+v\n", iface)

	wc, err := wireless.NewClient(iface)
	if err != nil {
		panic(err)
	}
	defer wc.Close()

	wc.ScanTimeout = time.Second * 3

	go scanEvents()

	aps, err := wc.Scan()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nScan results:\n%+v\n", aps)

	for _, ap := range aps {
		fmt.Printf("%s\n", ap.SSID)
	}

	net, err := wc.Networks()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nNetworks:\n%+v\n", net)

	didRemove := false

	for _, n := range net {
		if n.ID != 0 {
			fmt.Printf("Removing %s[%d]...\n", n.SSID, n.ID)
			_ = wc.RemoveNetwork(n.ID)
			didRemove = true
		}

		fmt.Printf("%s\n", n.SSID)
	}

	if didRemove {
		fmt.Printf("\nNetworks:\n%+v\n", net)
	}

	status, err := wc.Status()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nStatus:\n%+v\n", status)

	fmt.Println("\nAdding new network...")

	newNet := wireless.NewNetwork("KoalaArt", "pinkZebra37")
	newNet.ScanSSID = true
	newNet.KeyMgmt = "WPA-PSK FT-PSK WPA-PSK-SHA256"

	newNet, err = wc.AddNetwork(newNet)
	if err != nil {
		panic(err)
	}

	fmt.Println("Enabling new network...")

	_ = wc.EnableNetwork(newNet.ID)

	net, err = wc.Networks()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nNetworks:\n%+v\n", net)

	fmt.Println("Connecting to new network...")

	newNet, err = wc.Connect(newNet)
	if err != nil {
		_ = wc.DisableNetwork(newNet.ID)
		_ = wc.RemoveNetwork(newNet.ID)

		panic(err)
	}

	fmt.Printf("\nConnected to:\n%+v\n", newNet)

	status, err = wc.Status()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nStatus:\n%+v\n", status)
}

func scanEvents() {
	conn, err := wireless.Dial("wlan0")
	if err != nil {
		fmt.Printf("Unable to dial wlan0: %s", err)

		return
	}

	sub := conn.Subscribe(wireless.EventConnected, wireless.EventAuthReject, wireless.EventDisconnected)

	ev := <-sub.Next()
	switch ev.Name {
	case wireless.EventConnected:
		fmt.Println(ev.Arguments)
	case wireless.EventAuthReject:
		fmt.Println(ev.Arguments)
	case wireless.EventDisconnected:
		fmt.Println(ev.Arguments)
	}
}
