package httpArchive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/woodsaj/chromedriver_har/events"
	"log"
	"net/url"
	"strings"
	"time"
)

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

func EpochToTime(epoch float64) time.Time {
	return time.Unix(0, int64(epoch*1000)*int64(time.Millisecond))
}

func CreateHARFromEvents(chromeEvents []*events.ChromeEvent) (*HAR, error) {
	har := HAR{
		Log: Log{
			Version: "1.2",
			Creator: map[string]string{"name": "Raintank Chrome RemoteDebugging to HAR", "version": "0.1"},
			Pages:   make([]*Page, 0),
			Entries: make([]*Entry, 0),
		},
	}
	for _, e := range chromeEvents {
		switch e.Method {
		case "Page.frameStartedLoading":
			// new page being loaded.
			page := Page{
				StartedDateTime: e.Timestamp,
				PageTimings:     &PageTimings{},
			}
			har.Log.Pages = append(har.Log.Pages, &page)
			page.Id = har.CurrentPageId()
		case "Network.requestWillBeSent":
			if len(har.Log.Pages) < 1 {
				log.Fatal("Sending request object, but frame not started.")
			}
			//new HTTP request
			params := events.NetworkRequestWillBeSent{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}

			req := &Request{
				Method:    params.Request.Method,
				Url:       params.Request.Url,
				Timestamp: params.Timestamp,
			}

			req.BodySize = len(params.Request.PostData)

			req.ParseQueryString()
			entry := Entry{
				StartedDateTime: EpochToTime(params.WallTime), //epoch float64, eg 1440589909.59248
				RequestId:       params.RequestId,
				Pageref:         har.CurrentPageId(),
				Request:         req,
			}

			//TODO: check if ther is a redirectResponse
			if params.RedirectResponse != nil {
				lastEntry := har.GetEntryByRequestId(params.RequestId)
				lastEntry.RequestId = lastEntry.RequestId + "r"
				ProcessResponse(lastEntry, params.Timestamp, params.RedirectResponse)
				lastEntry.Response.RedirectUrl = params.Request.Url
				lastEntry.Timings.Receive = 0.0
			}

			har.Log.Entries = append(har.Log.Entries, &entry)

			page := har.CurrentPage()
			// if this is the primary page, set the Page.Title to the request URL
			if page.Title == "" {
				page.Title = req.Url
				page.Timestamp = params.Timestamp
			}

		case "Network.responseReceived":
			if len(har.Log.Pages) < 1 {
				//we havent loaded any pages yet.
				continue
			}
			params := events.NetworkResponseReceived{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}
			entry := har.GetEntryByRequestId(params.RequestId)
			if entry == nil {
				log.Fatal("got a response event with no matching request event.")
			}
			ProcessResponse(entry, params.Timestamp, params.Response)

		case "Network.dataReceived":
			if len(har.Log.Pages) < 1 {
				//we havent loaded any pages yet.
				continue
			}
			params := events.NetworkDataReceived{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}
			entry := har.GetEntryByRequestId(params.RequestId)
			if entry == nil {
				log.Fatal("got a response event with no matching request event.")
			}

			entry.Response.Content.Size += params.DataLength

		case "Network.loadingFinished":
			if len(har.Log.Pages) < 1 {
				//we havent loaded any pages yet.
				continue
			}
			params := events.NetworkLoadingFinished{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}
			entry := har.GetEntryByRequestId(params.RequestId)
			if entry == nil {
				log.Fatal("got a response event with no matching request event.")
			}
			entry.Response.BodySize = params.EncodedDataLength - int64(entry.Response.HeadersSize)
			entry.Response.Content.Compression = entry.Response.Content.Size - entry.Response.BodySize
			entry.Time = (params.Timestamp - entry.Request.Timestamp) * 1000
			entry.Timings.Receive = (entry.Timings.Receive + params.Timestamp*1000)
			if entry.Timings.Receive < 0.0 {
				entry.Timings.Receive = 0.0
			}
			if entry.Timings.Receive > entry.Time {
				entry.Timings.Receive = entry.Time
			}

		case "Page.loadEventFired":
			if len(har.Log.Pages) < 1 {
				//we havent loaded any pages yet.
				continue
			}
			params := events.PageLoadEventFired{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}
			page := har.CurrentPage()
			page.PageTimings.OnLoad = int64((params.Timestamp - page.Timestamp) * 1000)

		case "Page.domContentEventFired":
			if len(har.Log.Pages) < 1 {
				//we havent loaded any pages yet.
				continue
			}
			params := events.PageDomContentEventFired{}
			err := json.Unmarshal(e.Params, &params)
			if err != nil {
				log.Fatal(err)
			}
			page := har.CurrentPage()
			page.PageTimings.OnContentLoad = int64((params.Timestamp - page.Timestamp) * 1000)
		}

	}
	return &har, nil
}

func ProcessResponse(entry *Entry, timestamp float64, response *events.Response) {
	//Update the entry.Request with the new data available in this event.
	entry.Request.HttpVersion = response.Protocol
	entry.Request.Headers = parseHeaders(response.RequestHeaders)
	entry.Request.SetHeadersSize()
	entry.Request.ParseCookies()

	//create the entry.Response object
	resp := &Response{
		Status:      response.Status,
		StatusText:  response.StatusText,
		HttpVersion: entry.Request.HttpVersion,
		Headers:     parseHeaders(response.Headers),
		Timestamp:   timestamp,
	}
	resp.SetHeadersSize()
	resp.ParseCookies()
	entry.Response = resp

	entry.Response.Content = &ResponseContent{
		MimeType: response.MimeType,
	}

	blocked := response.Timing["dnsStart"]
	if blocked < 0.0 {
		blocked = 0.0
	}
	dns := response.Timing["dnsEnd"] - response.Timing["dnsStart"]
	if dns < 0.0 {
		dns = 0.0
	}
	connect := response.Timing["connectEnd"] - response.Timing["connectStart"]
	if connect < 0.0 {
		connect = 0.0
	}
	send := response.Timing["sendEnd"] - response.Timing["sendStart"]
	if send < 0.0 {
		send = 0.0
	}
	wait := response.Timing["receiveHeadersEnd"] - response.Timing["sendEnd"]
	if wait < 0.0 {
		wait = 0.0
	}
	ssl := response.Timing["sslEnd"] - response.Timing["sslStart"]
	if ssl < 0.0 {
		ssl = 0.0
	}
	timings := &Timings{
		Blocked: blocked,
		Dns:     dns,
		Connect: connect,
		Send:    send,
		Wait:    wait,
		Receive: 0.0 - (response.Timing["requestTime"]*1000 + response.Timing["receiveHeadersEnd"]),
		Ssl:     ssl,
	}
	entry.Timings = timings
	if entry.Timings.Receive < 0.0 {
		entry.Timings.Receive = 0.0
	}
}

type HAR struct {
	Log Log `json:"log"`
}

func (h *HAR) CurrentPageId() string {
	return fmt.Sprintf("page_%d", len(h.Log.Pages))
}

func (h *HAR) CurrentPage() *Page {
	if len(h.Log.Pages) < 1 {
		return nil
	}
	return h.Log.Pages[len(h.Log.Pages)-1]
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
	StartedDateTime time.Time    `json:"startedDateTime"`
	Id              string       `json:"id"`
	Title           string       `json:"title"`
	PageTimings     *PageTimings `json:"pageTimings"`
	Timestamp       float64      `json:"-"`
}

type PageTimings struct {
	OnContentLoad int64 `json:"onContentLoad"`
	OnLoad        int64 `json:"onLoad"`
}

type Entry struct {
	StartedDateTime time.Time              `json:"startedDateTime"`
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
	HeadersSize int            `json:"headersSize"`
	BodySize    int            `json:"bodySize"`
	Timestamp   float64        `json:"-"`
}

func (r *Request) ParseCookies() {
	r.Cookies = make([]*Cookie, 0)
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
	r.QueryString = make([]*QueryString, 0)
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

func (r *Request) SetHeadersSize() {
	var b bytes.Buffer
	reqUrl, err := url.Parse(r.Url)
	if err != nil {
		log.Fatal("unable to parse request URL")
	}

	b.Write([]byte(fmt.Sprintf("%s %s %s\r\n", r.Method, reqUrl.RequestURI(), r.HttpVersion)))
	for _, h := range r.Headers {
		b.Write([]byte(fmt.Sprintf("%s: %s\r\n", h.Name, h.Value)))
	}
	b.Write([]byte("\r\n"))
	r.HeadersSize = b.Len()
}

type Response struct {
	Status      int              `json:"status"`
	StatusText  string           `json:"statusText"`
	HttpVersion string           `json:"httpVersion"`
	Headers     []*Header        `json:"headers"`
	Cookies     []*Cookie        `json:"cookies"`
	Content     *ResponseContent `json:"content"`
	RedirectUrl string           `json:"redirectURL"`
	HeadersSize int              `json:"headersSize"`
	BodySize    int64            `json:"bodySize"`
	Timestamp   float64          `json:"-"`
}

func (r *Response) ParseCookies() {
	r.Cookies = make([]*Cookie, 0)
	//TODO: parse the response set-cookies header.
	return
}

func (r *Response) SetHeadersSize() {
	var b bytes.Buffer
	b.Write([]byte(fmt.Sprintf("%s %d %s\r\n", r.HttpVersion, r.Status, r.StatusText)))
	for _, h := range r.Headers {
		b.Write([]byte(fmt.Sprintf("%s: %s\r\n", h.Name, h.Value)))
	}
	b.Write([]byte("\r\n"))
	r.HeadersSize = b.Len()
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
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	HttpOnly bool      `json:"httpOnly"`
	Secure   bool      `json:"secure"`
}

type ResponseContent struct {
	Size        int64  `json:"size"`
	MimeType    string `json:"mimeType"`
	Compression int64  `json:"compression"`
}
