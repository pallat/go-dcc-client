// Copyright 2013-2015 go-diameter authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Diameter client example. This is by no means a complete client.
// The commands in here are not fully implemented. For that you have
// to read the RFCs (base and credit control) and follow the spec.

package main

import (
	"dcc/dictionary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/fiorix/go-diameter/diam"
	"github.com/fiorix/go-diameter/diam/avp"
	"github.com/fiorix/go-diameter/diam/datatype"
	"github.com/fiorix/go-diameter/diam/dict"
)

const (
	identity    = datatype.DiameterIdentity("jenkin13_OMR_TEST01")
	realm       = datatype.DiameterIdentity("dtac.co.th")
	vendorID    = datatype.Unsigned32(0)
	productName = datatype.UTF8String("omr")
	dtac        = "66816922438"
	dtn         = "66949014731"
)

func main() {
	ssl := flag.Bool("ssl", false, "connect using SSL/TLS")
	flag.Parse()
	if len(os.Args) < 2 {
		fmt.Println("Use: client [-ssl] host:port")
		return
	}

	dict.Default = dictionary.Load()
	// fmt.Println(dict.Default.String())

	// ALL incoming messages are handled here.
	sessionID := "dtac.co.th;OMR" + time.Now().Format("200601021504050000")
	msisdn := dtn
	diam.Handle("CEA", OnCEA(sessionID, msisdn))
	diam.HandleFunc("CCA", OnCCA)
	diam.HandleFunc("ALL", OnMSG) // Catch-all.
	// Connect using the default handler and base.Dict.
	addr := os.Args[len(os.Args)-1]
	log.Println("Connecting to", addr)
	var (
		c   diam.Conn
		err error
	)
	if *ssl {
		log.Println("going to DialTLS")
		c, err = diam.DialTLS(addr, "", "", nil, nil)
		log.Println("done to DialTLS")
	} else {
		log.Println("going to Dial")
		c, err = diam.Dial(addr, nil, nil)
		log.Println("done to Dial")
	}
	if err != nil {
		log.Fatal(err)
	}
	go NewClient(c)
	// Wait until the server kick us out.
	log.Println(<-diam.ErrorReports())
	log.Println(<-c.(diam.CloseNotifier).CloseNotify())
	log.Println("Server disconnected.")
}

// NewClient sends a CER to the server and then a DWR every 10 seconds.
func NewClient(c diam.Conn) {
	// Build CER
	m := diam.NewRequest(diam.CapabilitiesExchange, 0, nil)
	m.NewAVP(avp.OriginHost, avp.Mbit, 0, identity)
	m.NewAVP(avp.OriginRealm, avp.Mbit, 0, realm)
	laddr := c.LocalAddr()
	ip, _, _ := net.SplitHostPort(laddr.String())
	m.NewAVP(avp.HostIPAddress, avp.Mbit, 0, datatype.Address(net.ParseIP(ip)))
	m.NewAVP(avp.VendorID, avp.Mbit, 0, vendorID)
	m.NewAVP(avp.ProductName, 0, 0, productName)
	m.NewAVP(avp.SupportedVendorID, avp.Mbit, 0, datatype.Unsigned32(0))
	m.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(4))
	m.NewAVP(avp.OriginStateID, avp.Mbit, 0, datatype.Unsigned32(0))
	m.NewAVP(avp.AcctApplicationID, avp.Mbit, 0, datatype.Unsigned32(4))
	m.NewAVP(avp.FirmwareRevision, avp.Mbit, 0, datatype.Unsigned32(1))

	log.Printf("Sending message to %s", c.RemoteAddr().String())
	log.Println(m)
	// Send message to the connection
	if _, err := m.WriteTo(c); err != nil {
		log.Fatal("Write failed:", err)
	}
	// Send watchdog messages every 5 seconds
	for {
		time.Sleep(10 * time.Second)
		m = diam.NewRequest(diam.DeviceWatchdog, 0, nil)
		m.NewAVP(avp.OriginHost, avp.Mbit, 0, identity)
		m.NewAVP(avp.OriginRealm, avp.Mbit, 0, realm)
		m.NewAVP(avp.OriginStateID, avp.Mbit, 0, datatype.Unsigned32(rand.Uint32()))
		log.Printf("Sending message to %s", c.RemoteAddr().String())
		log.Println(m)
		if _, err := m.WriteTo(c); err != nil {
			log.Fatal("Write failed:", err)
		}
	}
}

const (
	BalanceInformation  = 21100
	AccessMethod        = 20340
	AccountQueryMethod  = 20346
	SSPTime             = 20386
	CallingPartyAddress = 20336
)

// OnCEA handles Capabilities-Exchange-Answer messages.
func OnCEA(sessionID string, msisdn string) diam.HandlerFunc {
	return func(c diam.Conn, m *diam.Message) {
		println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
		rc, err := m.FindAVP(avp.ResultCode)
		if err != nil {
			log.Fatal(err)
		}
		println("passed : m.FindAVP(avp.ResultCode)")
		if v, _ := rc.Data.(datatype.Unsigned32); v != diam.Success {
			log.Fatal("Unexpected response:", rc)
		}
		println("passed : rc.Data.(datatype.Unsigned32)")
		// Craft a CCR message.
		r := diam.NewRequest(diam.CreditControl, 4, nil)
		println("passed : diam.NewRequest")

		peerRealm, err := m.FindAVP(avp.OriginRealm) // You should handle errors.
		if err != nil {
			log.Fatal(err)
		}
		println("passed :FindAVP")
		fmt.Println(peerRealm.Data)

		// t, _ := time.Parse(time.UnixDate, "Feb 3, 2015 at 7:54pm (PST)")

		detail := []*diam.AVP{
			diam.NewAVP(CallingPartyAddress, avp.Mbit, 0, datatype.UTF8String(msisdn)),
			diam.NewAVP(AccessMethod, avp.Mbit, 0, datatype.Unsigned32(9)),
			diam.NewAVP(AccountQueryMethod, avp.Mbit, 0, datatype.Unsigned32(1)),
			diam.NewAVP(SSPTime, avp.Mbit, 0, datatype.Time(time.Now())),
		}

		r.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sessionID))
		r.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(4))
		r.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.OctetString("www.huawei.com")) //peerRealm.Data)
		r.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.OctetString("jenkin13_OMR_TEST01"))  //identity)
		r.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.OctetString("dtac.co.th"))          //realm)
		r.NewAVP(avp.CCRequestType, avp.Mbit, 0, datatype.Integer32(4))
		r.NewAVP(avp.SubscriptionID, avp.Mbit, 0, &diam.GroupedAVP{
			AVP: []*diam.AVP{
				diam.NewAVP(avp.SubscriptionIDType, avp.Mbit, 0, datatype.Integer32(0)),
				diam.NewAVP(avp.SubscriptionIDData, avp.Mbit, 0, datatype.UTF8String(msisdn)),
			},
		})
		r.NewAVP(avp.ServiceContextID, avp.Mbit, 0, datatype.UTF8String("QueryBalance@huawei.com"))
		r.NewAVP(avp.RequestedAction, avp.Mbit, 0, datatype.Integer32(2))
		r.NewAVP(avp.EventTimestamp, avp.Mbit, 0, datatype.Time(time.Now()))
		r.NewAVP(avp.ServiceIdentifier, avp.Mbit, 0, datatype.Unsigned32(0))
		r.NewAVP(avp.CCRequestNumber, avp.Mbit, 0, datatype.Unsigned32(0))
		r.NewAVP(avp.RouteRecord, avp.Mbit, 0, datatype.OctetString("10.89.111.40"))
		r.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.OctetString("cbp211"))
		r.NewAVP(avp.ServiceInformation, avp.Mbit, 0, &diam.GroupedAVP{
			AVP: []*diam.AVP{
				diam.NewAVP(BalanceInformation, avp.Mbit, 0, &diam.GroupedAVP{
					AVP: detail,
				}),
			},
		})

		// Add Service-Context-Id and all other AVPs...

		println("before :r.WriteTo(c)")
		fmt.Println(r)

		var returnCode int64

		returnCode, err = r.WriteTo(c)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println("after :r.WriteTo(c) ", returnCode)
	}
}

// OnCCA handles Credit-Control-Answer messages.
func OnCCA(c diam.Conn, m *diam.Message) {
	log.Printf("Receiving message from %s", c.RemoteAddr().String())
	log.Println(m)
}

// OnMSG handles all other messages and just print them.
func OnMSG(c diam.Conn, m *diam.Message) {
	log.Printf("Receiving message from %s", c.RemoteAddr().String())
	log.Println(m)
}
