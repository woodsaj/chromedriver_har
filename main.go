package main

import (
	"encoding/json"
	"fmt"
	"github.com/fedesog/webdriver"
	"github.com/woodsaj/chrome_perf_to_har/httpArchive"
	"github.com/woodsaj/chrome_perf_to_har/notifications"
	"log"
	"time"
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
	err := chromeDriver.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer chromeDriver.Stop()

	desired := webdriver.Capabilities{"loggingPrefs": logingPrefs}
	required := webdriver.Capabilities{}
	session, err := chromeDriver.NewSession(desired, required)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Delete()
)
	err = session.Url("https://www.raintank.io")
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
		fmt.Printf("ts: %s - Domain:%s - Event: %s - Params: %v\n", n.Timestamp, n.Domain, n.Event, n.Params)
	}

	httpArchive.CreateHARFromNotifications(events)
}
