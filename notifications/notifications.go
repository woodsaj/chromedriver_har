package notifications

import (
	"encoding/json"
	"fmt"
	"github.com/fedesog/webdriver"
	"strings"
	"time"
)

type ChromeNotification struct {
	Timestamp time.Time       `json:"-"`
	Domain    string          `json:"-"`
	Event     string          `json:"-"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	WebView   string          `json:"webview"`
}

type logWrapper struct {
	Message *ChromeNotification `json:"message"`
}

func NewFromLogEntry(entry webdriver.LogEntry) (*ChromeNotification, error) {
	l := logWrapper{}
	if err := json.Unmarshal([]byte(entry.Message), &l); err != nil {
		return nil, err
	}
	parts := strings.SplitN(l.Message.Method, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("unknwon method %s", l.Message.Method)
	}
	l.Message.Domain = parts[0]
	l.Message.Event = parts[1]
	l.Message.Timestamp = time.Unix(0, int64(entry.TimeStamp*1000)*int64(time.Microsecond))
	return l.Message, nil
}

type Request struct {
	Headers  map[string]string `json:"headers"`
	Method   string            `json:"method"`
	PostData string            `json:"postData"`
	Url      string            `json:"url"`
}

type Response struct {
	Headers            map[string]string  `json:"headers"`
	HeadersText        string             `json:"headersText"`
	MimeType           string             `json:"mimeType"`
	RequestHeaders     map[string]string  `json:"requestHeaders"`
	RequestHeadersText string             `json:"requestHeadersText"`
	Status             int                `json:"status"`
	StatusText         string             `json:"statusText"`
	Url                string             `json:"url"`
	Timing             map[string]float64 `json:"timing"`
	Protocol           string             `json:"protocol"`
}

type NetworkRequestWillBeSent struct {
	RequestId        string    `json:"requestId"`
	DocumentUrl      string    `json:"documentUrl"`
	Request          *Request  `json:"request"`
	WallTime         float64   `json:"wallTime"`
	Timestamp        float64   `json:"timestamp"`
	RedirectResponse *Response `json:"response"`
}

type NetworkResponseReceived struct {
	RequestId string    `json:"requestId"`
	Timestamp float64   `json:"timestamp"`
	Response  *Response `json:"response"`
}

type NetworkDataReceived struct {
	Timestamp         float64 `json:"timestamp"`
	RequestId         string  `json:"requestId"`
	EncodedDataLength int64   `json:"encodedDataLength"`
	DataLength        int64   `json:"dataLength"`
}

type NetworkLoadingFinished struct {
	Timestamp         float64 `json:"timestamp"`
	RequestId         string  `json:"requestId"`
	EncodedDataLength int64   `json:"encodedDataLength"`
}

type PageLoadEventFired struct {
	Timestamp float64 `json:"timestamp"`
}

type PageDomContentEventFired struct {
	Timestamp float64 `json:"timestamp"`
}
