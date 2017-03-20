package events

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/fedesog/webdriver"
)

type ChromeEvent struct {
	Timestamp time.Time       `json:"-"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	WebView   string          `json:"webview"`
}

type logWrapper struct {
	Message *ChromeEvent `json:"message"`
}

func NewFromLogEntries(entries []webdriver.LogEntry) ([]*ChromeEvent, error) {
	events := make([]*ChromeEvent, 0)
	for _, entry := range entries {
		w := logWrapper{}
		if err := json.Unmarshal([]byte(entry.Message), &w); err != nil {
			return events, err
		}
		w.Message.Timestamp = time.Unix(0, int64(entry.TimeStamp*1000)*int64(time.Microsecond))
		events = append(events, w.Message)

	}
	//if len(entries) != len(events) {
	//err := error{""}
	//}
	if len(events) == 0 {
		return events, errors.New("Failed to make events.")
	}
	return events, nil
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
	RedirectResponse *Response `json:"redirectResponse"`
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
