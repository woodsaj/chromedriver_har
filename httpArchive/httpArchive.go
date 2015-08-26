package httpArchive

import (
	"encoding/json"
	"fmt"
	"github.com/woodsaj/chrome_perf_to_har/notifications"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Notifications []*notifications.ChromeNotification

func (a Notifications) Len() int           { return len(a) }
func (a Notifications) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Notifications) Less(i, j int) bool { return a[i].Timestamp.Before(a[j].Timestamp) }

func parseHeaders(headers map[string]string) []*Header {
	h := make([]*Header, 0)
	for k, v := range headers {
		h = append(h, &Header{
			Name:  k,
			Value: v,
		})
	}
	return h
}

func CreateHARFromNotifications(events Notifications) (*HAR, error) {
	sort.Sort(events)
	har := HAR{
		Log: Log{
			Version: "1.2",
			Creator: map[string]string{"name": "Raintank Chrome RemoteDebugging to HAR", "version": "0.1"},
			Pages:   make([]*Page, 0),
			Entries: make([]*Entry, 0),
		},
	}
	for _, e := range events {
		switch e.Method {
		case "Page.frameStartedLoading":
			// new page being loaded.
			har.Log.Pages = append(har.Log.Pages, &Page{Id: har.CurrentPageId()})
		case "Network.requstWillBeSent":
			//new HTTP request
			params := notifications.NetworkRequestWillBeSent{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}
			req := &Request{
				Method: params.Request.Method,
				Url:    params.Request.Url,
			}

			req.BodySize = len(params.Request.PostData)

			req.ParseQueryString()
			entry := Entry{
				StartedDateTime: params.Timestamp, //epoch float64, eg 1440589909.59248
				RequestId:       params.RequestId,
				Pageref:         har.CurrentPageId(),
				Request:         req,
			}

			//TODO: check if ther is a redirectResponse

			har.Log.Entries = append(har.Log.Entries, &entry)

			// if this is the primary page, set the Page.Title to the request URL
			if har.Log.Pages[len(har.Log.Pages)-1].Title == "" {
				har.Log.Pages[len(har.Log.Pages)-1].Title = req.Url
			}
		case "Network.responseReceived":
			params := notifications.NetworkResponseReceived{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}
			entry := har.GetEntryByRequestId(params.RequestId)
			if entry == nil {
				log.Fatal("got a response event with no mathcing request event.")
			}

			//Update the entry.Request with the new data available in this event.
			entry.Request.Headers = parseHeaders(params.Response.RequestHeaders)
			entry.Request.HeaderSize = len(params.Response.RequestHeadersText)
			entry.Request.HttpVersion = params.Response.Protocol
			entry.Request.ParseCookies()

			//create the entry.Response object
			resp := &Response{
				Status:      params.Response.Status,
				StatusText:  params.Response.StatusText,
				HttpVersion: entry.Request.HttpVersion,
				Headers:     parseHeaders(params.Response.Headers),
				HeaderSize:  len(params.Response.HeadersText),
			}
			entry.Response = resp

			blocked := params.Response.Timing["dnsStart"]
			if blocked < 0.0 {
				blocked = 0.0
			}
			dns := params.Response.Timing["dnsEnd"] - params.Response.Timing["dnsStart"]
			if dns < 0.0 {
				dns = 0.0
			}
			connect := params.Response.Timing["connectEnd"] - params.Response.Timing["connectStart"]
			if connect < 0.0 {
				connect = 0.0
			}
			send := params.Response.Timing["sendEnd"] - params.Response.Timing["sendStart"]
			if send < 0.0 {
				send = 0.0
			}
			wait := params.Response.Timing["receiveHeadersEnd"] - params.Response.Timing["sendEnd"]
			if wait < 0.0 {
				wait = 0.0
			}
			ssl := params.Response.Timing["sslEnd"] - params.Response.Timing["sslStart"]
			if ssl < 0.0 {
				ssl = 0.0
			}
			timings := &Timings{
				Blocked: blocked,
				Dns:     dns,
				Connect: connect,
				Send:    send,
				Wait:    wait,
				Receive: 0.0,
				Ssl:     ssl,
			}
			entry.Timings = timings

		}

	}
	return &har, nil
}

type HAR struct {
	Log Log `json:"log"`
}

func (h *HAR) CurrentPageId() string {
	return fmt.Sprintf("page_%d", len(h.Log.Pages))
}

func (h *HAR) GetEntryByRequestId(id string) *Entry {
	for _, e := range h.Log.Entries {
		if e.RequestId == id {
			return e
		}
	}
	return nil
}

type Log struct {
	Version string            `json:"version"`
	Creator map[string]string `json:"creator"`
	Pages   []*Page           `json:"pages"`
	Entries []*Entry          `json:"entries"`
}

type Page struct {
	StartedDateTime float64            `json:"startedDateTime"`
	Id              string             `json:"id"`
	Title           string             `json:"title"`
	PageTimings     map[string]float64 `json:"pageTimings"`
}

type Entry struct {
	StartedDateTime float64                `json:"startedDateTime"`
	Time            float64                `json:"time"`
	Request         *Request               `json:"request"`
	Response        *Response              `json:"response"`
	Cache           map[string]interface{} `json:"cache"`
	Timings         *Timings               `json:"timings"`
	Pageref         string                 `json:"pageref"`
	RequestId       string                 `json:"-"`
}

type Timings struct {
	Blocked float64 `json:"blocked"`
	Dns     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	Ssl     float64 `json:"ssl"`
}

type Request struct {
	Method      string         `json:"method"`
	Url         string         `json:"url"`
	HttpVersion string         `json:"httpVersion"`
	Headers     []*Header      `json:"headers"`
	QueryString []*QueryString `json:"queryString"`
	Cookies     []*Cookie      `json:"cookies"`
	HeaderSize  int            `json:"headerSize"`
	BodySize    int            `json:"bodySize"`
}

func (r *Request) ParseCookies() {
	//check for a Cookie header
	for _, h := range r.Headers {
		if h.Name == "Cookie" {
			cookies := strings.Split(h.Value, ";")
			for _, c := range cookies {
				cookie := strings.TrimSpace(c)
				parts := strings.SplitN(cookie, "=", 2)
				r.Cookies = append(r.Cookies, &Cookie{Name: parts[0], Value: parts[1]})
			}
		}
	}
}

func (r *Request) ParseQueryString() {
	reqUrl, err := url.Parse(r.Url)
	if err != nil {
		log.Fatal("unable to parse request URL")
	}

	for k, v := range reqUrl.Query() {
		for _, value := range v {
			r.QueryString = append(r.QueryString, &QueryString{
				Name:  k,
				Value: value,
			})
		}
	}
}

type Response struct {
	Status      int              `json:"status"`
	StatusText  string           `json:"statusText"`
	HttpVersion string           `json:"httpVersion"`
	Headers     []*Header        `json:"headers"`
	Cookies     []*SetCookie     `json:"cookies"`
	Content     *ResponseContent `json:"content"`
	RedirectUrl string           `json:"redirectUrl"`
	HeaderSize  int              `json:"headerSize"`
	BodySize    int              `json:"bodySize"`
}

func (r *Response) ParseCookies() {
	//TODO: parse the response set-cookies header.
	return
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type QueryString struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SetCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	HttpOnly bool      `json:"httpOnly"`
	Secure   bool      `json:"secure"`
}

type ResponseContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
}
