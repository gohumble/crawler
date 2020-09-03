package crawler

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	cmongo "github.com/zolamk/colly-mongo-storage/colly/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
)

type crawlerService struct {
	resultChannel chan Result
	TTLMap        *TTLMap
	Dispatcher    *mux.Router
	Urls          map[string]func(w http.ResponseWriter, r *http.Request)
	Crawler       []Crawler
	GetCollection func(name string, opts ...*options.CollectionOptions) *mongo.Collection
	Collection    *mongo.Collection
	storage       cmongo.Storage
}

//var Service crawlerService

func NewService() *crawlerService {
	service := &crawlerService{Crawler: make([]Crawler, 0), TTLMap: NewMap(0, 60)}
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

type PageView struct {
	Id        primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Data      []byte              `bson:"data" json:"data"`
	Url       string              `bson:"url" json:"url"`
	Seed      string              `bson:"seed" json:"seed"`
	Timestamp primitive.Timestamp `bson:"timestamp" json:"timestamp"`
	Referer   int                 `bson:"referer" json:"referer"`
}

func (s *crawlerService) PageHandler(w http.ResponseWriter, r *http.Request) {
	dbc := connectDatabase(context.TODO())
	url := r.URL.Query().Get("url")
	c := dbc.Database("qmap").Collection("page_view").FindOne(context.TODO(), bson.M{"url": url})
	res := PageView{}
	c.Decode(&res)
	if res.Url == "" {
		w.WriteHeader(404)
		w.Write([]byte("NOT FOUND"))
		return
	}
	w.WriteHeader(200)
	if len(res.Data) > 0 {

		w.Write(res.Data)
	} else {
		w.Write([]byte("NO DATA"))
	}
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

func connectDatabase(ctx context.Context) *mongo.Client {
	var con string
	if Configuration.Username != "" && Configuration.Password != "" {
		con = fmt.Sprintf("mongodb://%s:%s@%s:%s", Configuration.Username, Configuration.Password, Configuration.Host, Configuration.Port)
	} else {
		con = fmt.Sprintf("mongodb://%s:%s", Configuration.Host, Configuration.Port)
	}
	client, err := mongo.NewClient(options.Client().ApplyURI(con),
		options.Client().SetConnectTimeout(0),
		options.Client().SetMaxConnIdleTime(0),
		options.Client().SetMaxPoolSize(0))
	if err != nil {
		panic(err)
	}
	err = client.Connect(ctx)
	return client
}
