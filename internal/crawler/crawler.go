package crawler

import (
	"context"
	"fmt"
	"github.com/bobesa/go-domain-util/domainutil"
	"github.com/gocolly/colly/v2"
	"github.com/orcaman/concurrent-map"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
	"time"
)

type Result struct {
	domain  string
	referer string
	HTML    []byte
}

type Crawler struct {
	resultChannel   chan Result
	mode            string
	Proto           string
	seed            string
	results         cmap.ConcurrentMap
	ttlMap          *TTLMap
	collector       *colly.Collector
	start           time.Time
	end             time.Time
	done            bool
	visited         int
	currentIndex    string
	requestCallback colly.RequestCallback
	*sync.Mutex
}

// proxy function for colly visit
func (cr *Crawler) Seed(URL string) {
	cr.seed = URL
}

// proxy function for colly visit
func (cr Crawler) Visit(URL string) error {
	return cr.collector.Visit(URL)
}

const (
	WHOIS = iota
	RDAP
)
const maxDepth int = 1

func New() Crawler {
	collector := colly.NewCollector(colly.MaxDepth(maxDepth), colly.Async(true))
	collector.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 5})
	c := Crawler{
		results:   cmap.New(),
		Mutex:     &sync.Mutex{},
		collector: collector,
		start:     time.Now(),
	}
	collector.SetRequestTimeout(time.Second * 30)
	return c
}

const RFC3339Millis = "2006-01-02T15:04:05.000Z07:00"

func (cr *Crawler) Crawl(collection *mongo.Collection) {
	// Do this on every found link
	cr.collector.OnHTML("a[href]", cr.onHTMLCallback)
	cr.collector.OnRequest(func(r *colly.Request) {
		url := r.URL.String()
		filter := bson.M{"url": url}
		d := collection.FindOne(context.TODO(), filter, options.FindOne().SetSort(bson.M{"_id": -1}))
		pageview := &PageView{}
		err := d.Decode(pageview)
		if err != nil {
			return
		}
		t := int64(pageview.Timestamp.T)
		if time.Now().Sub(time.Unix(t, 0)) < time.Hour {
			r.Abort()
		}
	})
	cr.collector.OnResponse(func(r *colly.Response) {
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		pv := PageView{Timestamp: primitive.Timestamp{T: uint32(time.Now().Unix())}, Url: r.Request.URL.String(), Data: r.Body, Seed: cr.seed}
		_, err := collection.InsertOne(ctx, pv)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Printf("inserted %s found from %s\n", pv.Url, pv.Seed)
	})
	cr.visited = 0
	err := cr.Visit(cr.Proto + cr.seed)
	if err != nil {
		cr.done = true
		fmt.Println(err)
		return
	}
	cr.collector.Wait()
	cr.done = true
	cr.end = time.Now()

	fmt.Printf("finished crawler for %s!\n", cr.seed)
	fmt.Printf("runtime %s!\n", cr.end.Sub(cr.start).String())
	time.Sleep(time.Second * 5)
}

func (cr *Crawler) onHTMLCallback(e *colly.HTMLElement) {
	ref := e.Attr("href")
	url := e.Request.AbsoluteURL(ref)
	visited, err := cr.collector.HasVisited(url)
	if err != nil {
		return
	}
	if domainutil.Domain(url) != "" && !visited && e.Request.Depth <= maxDepth {
		cr.currentIndex = url
		cr.Visit(url)
	}
}
