package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/apex/gateway"
	"github.com/carlmjohnson/feed2json"
	"golang.org/x/net/html/charset"
)

type MetOffice struct {
	// XMLName xml.Name `xml:"rss"`
	// Text    string   `xml:",chardata"`
	// Version string   `xml:"version,attr"`
	// Dc      string   `xml:"dc,attr"`
	Channel struct {
		// Text        string `xml:",chardata"`
		// Title       string `xml:"title"`
		// Link        string `xml:"link"`
		// Description string `xml:"description"`
		// Language    string `xml:"language"`
		// Copyright   string `xml:"copyright"`
		PubDate string `xml:"pubDate"`
		// Creator     string `xml:"creator"`
		Item []struct {
			// Text        string `xml:",chardata"`
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			GUID        string `xml:"guid"`
		} `xml:"item"`
	} `xml:"channel"`
}

type WarningsJSON struct {
	// warnings string `json:"warning"`
	Warnings []Item `json:"warnings"`
}
type Item struct {
	Level  string `json:"level"`
	Region string `json:"region"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Start  string `json:"start"`
	End    string `json:"end"`
}

type TrafficScotlandCI struct {
	XMLName xml.Name `xml:"rss"`
	// Text    string   `xml:",chardata"`
	// Version string   `xml:"version,attr"`
	// Georss  string   `xml:"georss,attr"`
	// Gml     string   `xml:"gml,attr"`
	Channel struct {
		// Text           string `xml:",chardata"`
		Title          string `xml:"title"`
		Description    string `xml:"description"`
		Link           string `xml:"link"`
		Language       string `xml:"language"`
		Copyright      string `xml:"copyright"`
		ManagingEditor string `xml:"managingEditor"`
		WebMaster      string `xml:"webMaster"`
		LastBuildDate  string `xml:"lastBuildDate"`
		Docs           string `xml:"docs"`
		Rating         string `xml:"rating"`
		Generator      string `xml:"generator"`
		Ttl            string `xml:"ttl"`
		Item           []struct {
			// Text        string `xml:",chardata"`
			Title       string `xml:"title"`
			Description string `xml:"description"`
			Link        string `xml:"link"`
			Point       string `xml:"point"`
			Author      string `xml:"author"`
			Comments    string `xml:"comments"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

type IncidentsJSON struct {
	// warnings string `json:"warning"`
	Incidents []Incident `json:"incidents"`
}
type Incident struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
	Point       string `json:"point"`
	Comments    string `json:"comments"`
	Date        string `json:"date"`
}

func main() {
	port := flag.Int("port", -1, "specify a port to use http rather than AWS Lambda")
	flag.Parse()
	listener := gateway.ListenAndServe
	portStr := ""
	if *port != -1 {
		portStr = fmt.Sprintf(":%d", *port)
		listener = http.ListenAndServe
		http.Handle("/", http.FileServer(http.Dir("./public")))
	}

	http.HandleFunc("/api/mo", metOffice)
	http.HandleFunc("/api/ts", trafficScotland)

	http.Handle("/api/feed", feed2json.Handler(
		feed2json.StaticURLInjector("http://www.metoffice.gov.uk/public/data/PWSCache/WarningsRSS/Region/UK"),
		nil, nil, nil, cacheControlMiddleware))
	log.Fatal(listener(portStr, nil))

}

func trafficScotland(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("https://trafficscotland.org/rss/feeds/currentincidents.aspx")
	if err != nil {
		log.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Status error: %v", resp.StatusCode)
	}

	var xmlIncidents TrafficScotlandCI
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	err = decoder.Decode(&xmlIncidents)
	if err != nil {
		fmt.Println(err)
	}

	var incidents IncidentsJSON

	// reRegion, _ := regexp.Compile("region=(.+)$")
	// ignore := "Wales|Northern Ireland|North West England|North East England|Yorkshire & Humber|West Midlands|East Midlands|East of England|South West England|London & South East England"

	// reDate, _ := regexp.Compile("valid from (.+) to (.+)$")
	// now := time.Now()
	// layout := "1504 Mon 02 Jan 2006"

	for _, r := range xmlIncidents.Channel.Item {
		// fmt.Printf("%v -> %v\n", k, r)

		incidents.Incidents = append(incidents.Incidents, Incident{
			Title:       r.Title,
			Description: r.Description,
			Link:        r.Link,
			Point:       r.Point,
			Date:        r.PubDate, //.Format("2006-01-02"),
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	enableCors(&w)
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(incidents); err != nil {
		panic(err)
	}

}

func metOffice(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://www.metoffice.gov.uk/public/data/PWSCache/WarningsRSS/Region/UK")
	if err != nil {
		log.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Status error: %v", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Read body: %v", err)
	}

	var result MetOffice
	xml.Unmarshal(data, &result)
	if nil != err {
		log.Fatalf("Error unmarshalling from XML: %v", err)
	}

	var warnings WarningsJSON

	reRegion, _ := regexp.Compile("region=(.+)$")
	ignore := "Wales|Northern Ireland|North West England|North East England|Yorkshire & Humber|West Midlands|East Midlands|East of England|South West England|London & South East England"

	reDate, _ := regexp.Compile("valid from (.+) to (.+)$")
	now := time.Now()
	layout := "1504 Mon 02 Jan 2006"

	for _, r := range result.Channel.Item {
		// fmt.Printf("%v -> %v\n", k, r)

		// extract region
		// --------------
		region := reRegion.FindStringSubmatch(r.GUID)[1]

		if strings.Contains(ignore, region) {
			fmt.Println("ignoring " + region)
			continue
		}

		// date handling
		// -------------
		dateRange := reDate.FindStringSubmatch(r.Description)
		// fmt.Println(dateRange[1], dateRange[2])

		start, err := time.Parse(layout, dateRange[1]+" "+fmt.Sprint(now.Year()))
		if err != nil {
			fmt.Println(err)
		}

		end, err := time.Parse(layout, dateRange[2]+" "+fmt.Sprint(now.Year()))
		if err != nil {
			fmt.Println(err)
		}
		// fmt.Println(start, end)

		// for d := start; d.After(end) == false; d = d.AddDate(0, 0, 1) {
		// fmt.Println(d.Format("2006-01-02"))

		// populate warning(s)
		// -------------------

		level := strings.Fields(r.Title)[0]

		warnings.Warnings = append(warnings.Warnings, Item{
			Level:  level,
			Region: region,
			URL:    r.GUID,
			Title:  r.Title,
			Text:   r.Description,
			Start:  start.Format("2006-01-02 15:04"),
			End:    end.Format("2006-01-02 15:04"),
		})
		// }
	}

	// fmt.Println(warnings)

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	enableCors(&w)
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(warnings); err != nil {
		panic(err)
	}
}

func cacheControlMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		h.ServeHTTP(w, r)
	})
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}
