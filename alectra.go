package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/antchfx/xmlquery"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	log "github.com/sirupsen/logrus"
)

var (
	ErrFailedLogin = fmt.Errorf("Failed login")
	alectraLogger  = log.WithField("Alectra", nil)
)

func alectraScrape(tlsConfig *tls.Config) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	err = alectraLogin(client)
	if err != nil {
		return "", err
	}

	downloadKey, err := alectraKey(client)
	if err != nil {
		return "", err
	}

	greenButtonXML, err := alectraXML(client, downloadKey)
	if err != nil {
		return "", err
	}

	readings, err := alectraParseXML(greenButtonXML)
	if err != nil {
		return "", err
	}

	importInfluxDB(tlsConfig, readings)

	return "", nil
}

func importInfluxDB(tlsConfig *tls.Config, readings []IntervalReading) {
	// Create a client
	// You can generate an API Token from the "API Tokens Tab" in the UI
	client := influxdb2.NewClientWithOptions(alectraConfig.InfluxAddress, alectraConfig.InfluxPass,
		influxdb2.DefaultOptions().
			SetUseGZip(true).
			SetTLSConfig(tlsConfig))
	// always close client at the end
	defer client.Close()

	// get non-blocking write client
	writeAPI := client.WriteAPI("home", "alectra")

	for _, reading := range readings {
		_ = reading
		p := influxdb2.NewPointWithMeasurement("meter").
			//AddTag("unit", "temperature").
			AddField("cost", reading.Cost).
			AddField("energy", reading.Value).
			AddField("duration", int32(time.Duration(reading.TimePeriod.Duration).Seconds())).
			SetTime(time.Time(reading.TimePeriod.Start)) //time.Now())
		// write point asynchronously
		writeAPI.WritePoint(p)
	}
	// p := influxdb2.NewPoint("stat",
	// 	map[string]string{"unit": "temperature"},
	// 	map[string]interface{}{"avg": 24.5, "max": 45},
	// 	time.Now())
	// // write point asynchronously
	// writeAPI.WritePoint(p)
	// // create point using fluent style
	// p = influxdb2.NewPointWithMeasurement("stat").
	// 	AddTag("unit", "temperature").
	// 	AddField("avg", 23.2).
	// 	AddField("max", 45).
	// 	SetTime(time.Now())
	// // write point asynchronously
	// writeAPI.WritePoint(p)
	// Flush writes
	writeAPI.Flush()
}

type EspiDuration time.Duration

func (a *EspiDuration) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s int64
	if err := d.DecodeElement(&s, &start); err != nil {
		log.Infof("VP was here >> %s", err)
		return err
	}
	*a = EspiDuration(s) * EspiDuration(time.Second)
	return nil
}

// func (a EspiDuration) MarshalJSON() ([]byte, error) {
// 	//return []byte(time.Duration(a).String()), nil
// 	return time.Duration(a).MarshalJSON()
// }

type EspiTime time.Time

func (a *EspiTime) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s int64
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}

	*a = EspiTime(time.Unix(s, 0))
	return nil
}

func (a EspiTime) MarshalJSON() ([]byte, error) {
	//return []byte(time.Time(a).String()), nil
	return time.Time(a).MarshalJSON()
}

type TimePeriod struct {
	//<espi:timePeriod><espi:duration>3600</espi:duration><espi:start>1659675600</espi:start></espi:timePeriod>
	Duration EspiDuration `xml:"duration"`
	Start    EspiTime     `xml:"start"`
}

type IntervalReading struct {
	Cost       uint32     `xml:"cost"`  //<espi:cost>0</espi:cost>
	Value      uint32     `xml:"value"` //<espi:value>0</espi:value>
	TimePeriod TimePeriod `xml:"timePeriod"`
}

func alectraParseXML(greenButtonXML []byte) ([]IntervalReading, error) {
	doc, err := xmlquery.Parse(bytes.NewReader(greenButtonXML))
	if err != nil {
		return nil, err
		//panic(err)
	}

	readings := make([]IntervalReading, 0)

	for _, n := range xmlquery.Find(doc, "/feed/entry/content/espi:IntervalBlock/espi:IntervalReading") {
		var reading IntervalReading
		err = xml.Unmarshal([]byte(n.OutputXML(true)), &reading)
		if err != nil {
			return nil, err
		}
		if reading.Cost == 0 && reading.Value == 0 {
			// Seems like fake data, data not ready
			continue
		}
		readings = append(readings, reading)
		//fmt.Printf("#%d %s\n", i, n.OutputXML(true))
	}

	b, _ := json.MarshalIndent(readings, "   ", "   ")

	alectraLogger.Infof("Parsed XML into structs: %s", b)
	return readings, nil
}

func alectraXML(client *http.Client, downloadKey string) ([]byte, error) {
	req, err := http.NewRequest("POST", alectraConfig.UrlAlectra+downloadKey, nil)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	alectraLogger.Infof("Downloaded XML")
	//fmt.Println(string(body))

	return body, nil
}

func alectraLogin(client *http.Client) error {
	urlLogin := alectraConfig.UrlAlectra + "/app/capricorn" //?para=index"
	//urlKey := urlAlectra + "/app/capricorn?para=greenButtonDownload"

	req, err := http.NewRequest("POST", urlLogin, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.URL.RawQuery = url.Values{
		"para":            {"index"},
		"password":        {alectraConfig.Password},
		"loginBy":         {"email"},
		"accessEmail":     {alectraConfig.UserID},
		"password1":       {alectraConfig.Password},
		"rememberMyEmail": {"N"},
	}.Encode()

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return ErrFailedLogin // This doesnt actually work, need to look at the text in the return page
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if strings.Contains(string(body), "We encountered a problem") {
		return ErrFailedLogin
	}

	alectraLogger.Infof("Login Success")
	return nil
}

// fmt.Println(res.Cookies())
// fmt.Printf("Status code %d\n", res.StatusCode)
// _ = body
// fmt.Println(string(body))

const Day = time.Hour * time.Duration(24)

func alectraKey(client *http.Client) (string, error) {
	urlKey := alectraConfig.UrlAlectra + "/app/capricorn" //?para=greenButtonDownload"
	req, err := http.NewRequest("POST", urlKey, nil)
	if err != nil {
		return "", err
	}

	currentTime := time.Now()
	// beforeTime := currentTime.Add(time.Duration(-1*24*30*12) * time.Hour)
	// beforeTime := currentTime.Add(time.Duration(-200) * Day) //
	beforeTime := currentTime.Add(time.Duration(-2) * Day)

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.URL.RawQuery = url.Values{
		"para":                {"greenButtonDownload"},
		"GB_iso_fromDate":     {beforeTime.Format("2006-01-02")},  //f"{year}-{month:02d}-{day:02d}",
		"GB_iso_toDate":       {currentTime.Format("2006-01-02")}, //f"{year}-{month:02d}-{day:02d}",
		"downloadConsumption": {""},
		"tab":                 {"GBDL"},
		"hourlyOrDaily":       {"Hourly"},
		"GB_fromDate":         {fmt.Sprintf("%02d%%2F%02d%%2F%d", beforeTime.Month(), beforeTime.Day(), beforeTime.Year())},    //f"{month:02d}%2F{day:02d}%2F{year}",
		"GB_month_from":       {beforeTime.Format("01")},                                                                       //f"{month:02d}",
		"GB_day_from":         {beforeTime.Format("02")},                                                                       //f"{day:02d}",
		"GB_year_from":        {beforeTime.Format("2006")},                                                                     //f"{year}",
		"GB_toDate":           {fmt.Sprintf("%02d%%2F%02d%%2F%d", currentTime.Month(), currentTime.Day(), currentTime.Year())}, //f"{month:02d}%2F{day:02d}%2F{year}",
		"GB_month_to":         {currentTime.Format("01")},                                                                      //f"{month:02d}",
		"GB_day_to":           {currentTime.Format("02")},                                                                      //f"{day:02d}",
		"GB_year_to":          {currentTime.Format("2006")},                                                                    //f"{year}"
	}.Encode()

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", ErrFailedLogin // This doesnt actually work, need to look at the text in the return page
	}

	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		return "", err
	}

	downloadXmlKey, ok := doc.Find("[name=\"downloadXml\"]").Attr("action") //
	if !ok {
		return "", fmt.Errorf("downloadXML key not found")
	}

	alectraLogger.Infof("Found download key")
	return downloadXmlKey, nil
}
