package main

import (
	"encoding/json"
	"github.com/fedesog/webdriver"
	"github.com/woodsaj/chromedriver_har/events"
	"github.com/woodsaj/chromedriver_har/httpArchive"
	"io/ioutil"
	"log"
)

var logingPrefs = map[string]webdriver.LogLevel{
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

	_, err = session.Log("performance")
	if err != nil {
		log.Fatal(err)
	}

	err = session.Url("https://www.raintank.io")
	if err != nil {
		log.Fatal(err)
	}

	logs, err := session.Log("performance")
	if err != nil {
		log.Fatal(err)
	}

	e, err := events.NewFromLogEntries(logs)
	if err != nil {
		log.Fatal(err)
	}

	har, err := httpArchive.CreateHARFromEvents(e)
	if err != nil {
		log.Fatal(err)
	}
	harJson, err := json.Marshal(har)
	eventJson, err := json.Marshal(e)

	// write out the HAR file and the RAW chromeEvens to file
	ioutil.WriteFile("./chromdriver.har", harJson, 0644)
	ioutil.WriteFile("./chromdriver.json", eventJson, 0644)
}
