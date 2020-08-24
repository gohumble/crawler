package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	rdap2 "github.com/gohumble/crawler/internal/rdap"
	"github.com/gohumble/crawler/internal/whois"
	"github.com/gorilla/mux"
	cmongo "github.com/zolamk/colly-mongo-storage/colly/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type crawlerService struct {
	resultChannel chan Result
	TTLMap        *TTLMap
	Dispatcher    *mux.Router
	Urls          map[string]func(w http.ResponseWriter, r *http.Request)
	Crawler       []Crawler
	GetCollection func(name string, opts ...*options.CollectionOptions) *mongo.Collection
	storage       cmongo.Storage
}

//var Service crawlerService

func NewService() *crawlerService {
	service := &crawlerService{Crawler: make([]Crawler, 0), TTLMap: NewMap(0, 60)}
	service.connectDatabase()
	service.resultChannel = make(chan Result, 100)
	service.Urls = make(map[string]func(w http.ResponseWriter, r *http.Request), 0)
	service.Dispatcher = mux.NewRouter()
	service.Dispatcher.HandleFunc("/", service.PageHandler).Methods("GET")
	go service.observer()
	fmt.Println("started service")
	return service
}

func remove(s []Crawler, i int) []Crawler {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func (s *crawlerService) worker(c Crawler) {
	var numWorker = 2
	for i := 0; i <= numWorker; i++ {
		fmt.Printf("result worker %d started\n", i)
		go func(i int) {
			for {
				res := <-s.resultChannel

				ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
				pv := PageView{Timestamp: primitive.Timestamp{T: uint32(time.Now().Unix())}, Url: res.referer, Data: res.HTML, Seed: c.seed}
				_, err := s.GetCollection("page_view").InsertOne(ctx, pv)
				if err != nil {
					panic(err)
				}
			}
		}(i)
	}

}
func (s *crawlerService) observer() {
	for {
		for i, c := range s.Crawler {
			if c.done {
				fmt.Printf("crawler is done! removing %s\n", c.seed)
				s.Crawler = remove(s.Crawler, i)
				continue
			}
		}
	}
}

func (s crawlerService) hasResult(url string) bool {
	for _, c := range s.Crawler {
		if _, e := c.results.Get(url); e {
			return true
		}
	}
	return false
}

func (s *crawlerService) RdapHandler(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")

	res := make([]rdap2.Result, 0)
	filter := bson.D{}
	if domain := r.URL.Query().Get("domain"); domain != "" {
		filter = bson.D{{"domain", r.URL.Query().Get("domain")}}
	}
	//optio := options.Find().SetLimit(10)
	limitInt, err := strconv.ParseInt(limit, 10, 64)
	limitOption := options.Find().SetLimit(limitInt)
	offsetInt, err := strconv.ParseInt(offset, 10, 64)
	offsetOption := options.Find().SetSkip(offsetInt)
	sort := make(map[string]interface{}, 0)
	sort["timestamp"] = -1
	sortOtion := options.Find().SetSort(sort)

	c, err := s.GetCollection("rdap").Find(context.TODO(), filter, limitOption, offsetOption, sortOtion)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = c.All(context.TODO(), &res)

	if err != nil {
		fmt.Println(err)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	resj, _ := json.Marshal(res)
	w.WriteHeader(200)
	w.Write(resj)
}

type PageView struct {
	Id        primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Data      []byte              `bson:"data" json:"data"`
	Url       string              `bson:"url" json:"url"`
	Seed      string              `bson:"seed" json:"seed"`
	Timestamp primitive.Timestamp `bson:"timestamp" json:"timestamp"`
	Referer   int                 `bson:"referer" json:"referer"`
}

func (s *crawlerService) PageHandler(w http.ResponseWriter, r *http.Request) {

	url := r.URL.Query().Get("url")
	c := s.GetCollection("page_view").FindOne(context.TODO(), bson.M{"url": url})
	res := PageView{}
	c.Decode(&res)
	if res.Url == "" {
		w.WriteHeader(404)
		w.Write([]byte("NOT FOUND"))
		return
	}
	w.WriteHeader(200)
	if len(res.Data) > 0 {
		s := string(res.Data)
		s = strings.Replace(s, `<a href="`, `<a href="http://localhost:8080/?url=`+url, -1)
		w.Write([]byte(s))
	} else {
		w.Write([]byte("NO DATA"))
	}
}

func (s *crawlerService) WhoisHandler(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")

	res := make([]whois.Result, 0)
	filter := bson.D{}
	if domain := r.URL.Query().Get("domain"); domain != "" {
		filter = bson.D{{"domain", r.URL.Query().Get("domain")}}
	}
	//optio := options.Find().SetLimit(10)
	limitInt, err := strconv.ParseInt(limit, 10, 64)
	limitOption := options.Find().SetLimit(limitInt)
	offsetInt, err := strconv.ParseInt(offset, 10, 64)
	offsetOption := options.Find().SetSkip(offsetInt)
	sort := make(map[string]interface{}, 0)
	sort["timestamp"] = -1
	sortOption := options.Find().SetSort(sort)

	c, err := s.GetCollection("whois").Find(context.TODO(), filter, limitOption, offsetOption, sortOption)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = c.All(context.TODO(), &res)

	if err != nil {
		fmt.Println(err)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	resj, _ := json.Marshal(res)
	w.WriteHeader(200)
	w.Write(resj)
}

func (s *crawlerService) CrawlerHandler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("url")
	URL := p
	log.Printf("start crawler for %s", URL)
	cr := New()
	if URL == "" {
		w.Write([]byte("missing URL argument"))
		log.Println("missing URL argument")
		return
	}
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
	cr.seed = URL

	go cr.Crawl(s.GetCollection("page_view"))
	s.Crawler = append(s.Crawler, cr)
	//s.AddFunction(cr.Handler, p)
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf("started crawling.\nseed -> %s", cr.seed)))
}

func (s *crawlerService) AddFunction(handler func(w http.ResponseWriter, r *http.Request), seed string) {
	s.Urls[seed] = handler // Add the handler to our map
}

func (s *crawlerService) RemoveFunction(seed string) {
	delete(s.Urls, seed)
}

func (s *crawlerService) ProxyCall(w http.ResponseWriter, r *http.Request, seed string) {
	if s.Urls[seed] != nil {
		s.Urls[seed](w, r) //proxy the call
	} else {
		w.Write([]byte("no PROXY"))
	}
}

func (s *crawlerService) connectDatabase() {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017/"))
	if err != nil {
		panic(err)
	}
	err = client.Connect(context.TODO())
	s.GetCollection = client.Database("testing").Collection
	/*ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
	cur, err := collection.Find(ctx, bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var result bson.M
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(result)
	}
	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}*/
	/* INSERT
	collection := client.Database("testing").Collection("numbers")
	res, err := collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
	id := res.InsertedID
	fmt.Println(id)
	*&
	*/
}
