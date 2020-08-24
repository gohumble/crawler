package main

import (
	"fmt"
	"github.com/CaliDog/certstream-go"
	"github.com/gohumble/crawler/internal/crawler"
	"net/http"
	"strings"
	"time"
)

func main() {

	s := crawler.NewService()
	stream, errch := certstream.CertStreamEventStream(true)
	go func() {
		for {
			select {
			case jq := <-stream:
				commonName, err := jq.String("data", "leaf_cert", "subject", "CN")

				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				cr := crawler.New()

				URL := strings.Replace(strings.Replace(commonName, "*.", "", 1), "www.", "", 1)

				if !strings.HasPrefix(URL, "http://") && !strings.HasPrefix(URL, "https://") {
					URL = "http://" + URL
				}
				if strings.HasPrefix(URL, "http://") {
					URL = strings.Replace(URL, "http://", "", 1)
					cr.Proto = "http://"
				} else if strings.HasPrefix(URL, "https://") {
					URL = strings.Replace(URL, "https://", "", 1)
					cr.Proto = "https://"
				}
				cr.Seed(URL)
				go cr.Crawl(s.Collection)
				s.Crawler = append(s.Crawler, cr)
				time.Sleep(time.Second * 3)
			case err := <-errch:
				fmt.Println(err.Error())

			}
		}
	}()
	http.ListenAndServe(":8080", s.Dispatcher)
}
