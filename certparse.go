package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/html/atom"

	database "github.com/foosinn/certparse/db"
	"github.com/foosinn/certparse/title"
)

var (
	exiting bool
	last50  [50]string
	db      *gorm.DB
	end     sync.WaitGroup
)

type (
	storer interface {
		Store(db *gorm.DB)
	}
)

func main() {
	ctx := context.Background()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// get database
	db = database.Init()
	defer db.Close()

	// initialize channels
	checkLimit := make(chan bool, 10000)
	certChan := make(chan string, 10000)
	storeChan := make(chan storer, 100)
	sigsChan := make(chan os.Signal, 1)
	for i := 0; i < 10000; i++ {
		checkLimit <- true
	}

	// metrics
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "certparse_running: %d\n", 10000-len(checkLimit))
		fmt.Fprintf(w, "certparse_certbuffer: %d\n", len(certChan))
		fmt.Fprintf(w, "certparse_storebuffer: %d\n", len(storeChan))
		fmt.Fprintf(w, "certparse_goroutines: %d\n", runtime.NumGoroutine())
	})
	go func() {
		err := http.ListenAndServe(":9123", nil)
		if err != nil {
			logrus.Fatal(err)
		}
	}()

	// get logclient
	lc, err := client.New(
		"https://ct.googleapis.com/rocketeer",
		&http.Client{},
		jsonclient.Options{},
	)
	if err != nil {
		logrus.Fatal(err)
	}

	// exit handler
	go func() {
		signal.Notify(sigsChan, os.Interrupt)
		<-sigsChan
		logrus.Warnln("Exiting...")
		exiting = true
	}()

	// retrive certs
	end.Add(1)
	go retriveCerts(ctx, lc, certChan)

	// store certs
	end.Add(1)
	go func() {
		defer end.Done()
		for s := range storeChan {
			logrus.Printf("%+v", s)
			s.Store(db)
		}
	}()

	// get certinfo
	end.Add(1)
	go func() {
		defer end.Done()
		defer close(storeChan)
		storeWg := sync.WaitGroup{}

		for record := range certChan {
			storeWg.Add(1)
			if strings.HasPrefix(record, "*.") {
				record = "www." + record[2:]
			}
			if known(record) {
				continue
			}
			<-checkLimit
			go func(record string) {
				defer storeWg.Done()
				defer func() { checkLimit <- true }()
				s, err := title.GetInfo(record, []atom.Atom{atom.Title, atom.Meta})
				if err != nil {
					logrus.Error(err)
				} else {
					storeChan <- &s
				}

			}(record)
		}
	}()
	end.Wait()

}

func known(record string) bool {
	for _, known := range last50 {
		if known == record {
			return true
		}
	}
	for i := 48; i >= 0; i-- {
		last50[i+1] = last50[i]
	}
	last50[0] = record
	return false
}

func retriveCerts(ctx context.Context, lc *client.LogClient, consumer chan string) {
	defer end.Done()
	defer close(consumer)
	for start := 0; true; start += 10000 {
		re, err := lc.GetRawEntries(ctx, int64(start), int64(start+9999))
		if err != nil {
			logrus.Fatal("Unable to download certs.")
		}
		for i, entry := range re.Entries {
			if exiting {
				return
			}
			logEntry, err := ct.LogEntryFromLeaf(int64(start+i), &entry)
			if err != nil {
				continue
			}
			if logEntry.X509Cert != nil {
				cert, err := x509.ParseCertificate(logEntry.X509Cert.Raw)
				if err != nil {
					logrus.Error(err)
					continue
				}
				for _, record := range cert.DNSNames {
					consumer <- record
				}
			}
		}
	}
}
