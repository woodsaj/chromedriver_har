package main

import (
	"encoding/json"
	"github.com/fedesog/webdriver"
	"github.com/woodsaj/chromedriver_har/httpArchive"
	"github.com/woodsaj/chromedriver_har/notifications"
	"io/ioutil"
	"log"
	"net/url"
)

var logingPrefs = map[string]webdriver.LogLevel{
	"driver":      webdriver.LogInfo,
	"browser":     webdriver.LogAll,
	"performance": webdriver.LogInfo,
}

type PerfLog struct {
	Method  string                     `json:"method"`
	Params  map[string]json.RawMessage `json:"params"`
	WebView string                     `json:"webview"`
}

func main() {
	chromeDriver := webdriver.NewChromeDriver("/usr/local/bin/chromedriver")
	u, err := url.Parse("http://localhost:9515")
	if err != nil {
		log.Fatal(err)
	}
	chromeDriver.SetUrl(u)

	desired := webdriver.Capabilities{"loggingPrefs": logingPrefs}
	required := webdriver.Capabilities{}
	session, err := chromeDriver.NewSession(desired, required)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Delete()

	_, err = session.Log("performance")
	if err != nil {
		log.Fatal(err)
	}

	err = session.Url("https://www.google.com")
	if err != nil {
		log.Fatal(err)
	}

	logs, err := session.Log("performance")
	if err != nil {
		log.Fatal(err)
	}
	events := make([]*notifications.ChromeNotification, 0)
	for _, l := range logs {
		n, err := notifications.NewFromLogEntry(l)
		if err != nil {
			log.Fatal(err)
		}
		events = append(events, n)
		//fmt.Printf("ts: %s - Domain:%s - Event: %s - Params: %v\n", n.Timestamp, n.Domain, n.Event, n.Params)
	}

	har, err := httpArchive.CreateHARFromNotifications(events)
	if err != nil {
		log.Fatal(err)
	}
	harJson, err := json.Marshal(har)
	eventJson, err := json.Marshal(events)

	ioutil.WriteFile("/tmp/chromdriver.har", harJson, 0644)
	ioutil.WriteFile("/tmp/chromdriver.json", eventJson, 0644)
}
