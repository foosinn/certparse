package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"

	database "github.com/foosinn/certparse/db"
	"github.com/foosinn/certparse/wp"
)

var (
	wpWaitGroup sync.WaitGroup
	Wp          map[string]interface{}
	last50      [50]string
	db          *gorm.DB
)

func main() {
	ctx := context.Background()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// get database
	db = database.Init()
	defer db.Close()

	// init vars
	Wp = map[string]interface{}{}
	wpWaitGroup = sync.WaitGroup{}

	// initialize channels
	checkLimit := make(chan bool, 10000)
	certChan := make(chan string, 10000)
	storeChan := make(chan wp.WpInfo, 100)
	for i := 0; i < 10000; i++ {
		checkLimit <- true
	}

	// metrics
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "certparse_found: %d\n", len(Wp))
		fmt.Fprintf(w, "certparse_running: %d\n", 10000-len(checkLimit))
		fmt.Fprintf(w, "certparse_certbuffer: %d\n", len(certChan))
		fmt.Fprintf(w, "certparse_storebuffer: %d\n", len(storeChan))
		fmt.Fprintf(w, "certparse_goroutines: %d\n", runtime.NumGoroutine())
	})
	go http.ListenAndServe(":9123", nil)

	// get logclient
	lc, err := client.New(
		"https://ct.googleapis.com/rocketeer",
		&http.Client{},
		jsonclient.Options{},
	)
	if err != nil {
		logrus.Fatal("%v", err)
	}

	// parse certs
	go retriveCerts(ctx, lc, certChan)
	go storeCerts(storeChan)
	recordCounter := 0
	for record := range certChan {
		recordCounter++
		if strings.HasPrefix(record, "*.") {
			record = "www." + record[2:]
		}
		if known(record) {
			continue
		}
		<-checkLimit
		go func(record string) {
			defer func() {
				checkLimit <- true
			}()
			info, err := wp.Check(record)
			if err != nil {
				// logrus.Error(err)
			} else if info.Name != "" {
				storeChan <- info
			}

		}(record)
	}

}

func storeCerts(infos chan wp.WpInfo) {
	for info := range infos {
		Wp[info.Name] = nil
		logrus.Info(info)

		tags := []database.Tag{}
		for _, tagName := range info.Tags {
			tag := database.Tag{}
			db.FirstOrInit(&tag, database.Tag{Name: tagName})
			tags = append(tags, tag)
		}
		categories := []database.Category{}
		for _, categoryName := range info.Categories {
			category := database.Category{}
			db.FirstOrInit(&category, database.Category{Name: categoryName})
			categories = append(categories, category)
		}
		site := database.Site{}
		db.FirstOrInit(&site, database.Site{URL: info.URL, Name: info.Name})
		db.Model(&site).Association("Tags").Append(tags)
		db.Model(&site).Association("Categories").Append(categories)
		db.Save(&site)
	}
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
	for start := 0; true; start += 10000 {
		re, err := lc.GetRawEntries(ctx, int64(start), int64(start+9999))
		if err != nil {
			logrus.Fatal("Unable to download certs.")
		}
		for i, entry := range re.Entries {
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
