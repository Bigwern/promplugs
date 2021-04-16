package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/jinzhu/configor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	temperature = prometheus.NewGauge(
		prometheus.GaugeOpts{
			// Namespace: "golang",
			Name: "shellyS_temperature",
			Help: "Temperature gauge",
		})

	powermeter = prometheus.NewGauge(
		prometheus.GaugeOpts{
			// Namespace: "golang",
			Name: "shellyS_meter_power",
			Help: "Metered power",
		})

	shellyS_unixtime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			// Namespace: "golang",
			Name: "shellyS_unixtime",
			Help: "Last seen unixtime on shelly",
		})

	shellyS_rssi = prometheus.NewGauge(
		prometheus.GaugeOpts{
			// Namespace: "golang",
			Name: "shellyS_rssi",
			Help: "Wifi strength to shelly",
		})

	Config = struct {
		Shelly struct {
			Url      string `required:"true" env:"SHELLYURL"`
			User     string `default:"admin"`
			Password string `required:"false" env:"SHELLYPW"`
		}
	}{}

	client = &http.Client{}

	dat PlugS2point5

	req = &http.Request{}
)

type PlugS2point5 struct {
	WifiSta struct {
		Connected bool   `json:"connected"`
		Ssid      string `json:"ssid"`
		IP        string `json:"ip"`
		Rssi      int    `json:"rssi"`
	} `json:"wifi_sta"`
	Cloud struct {
		Enabled   bool `json:"enabled"`
		Connected bool `json:"connected"`
	} `json:"cloud"`
	Mqtt struct {
		Connected bool `json:"connected"`
	} `json:"mqtt"`
	Time          string `json:"time"`
	Unixtime      int    `json:"unixtime"`
	Serial        int    `json:"serial"`
	HasUpdate     bool   `json:"has_update"`
	Mac           string `json:"mac"`
	CfgChangedCnt int    `json:"cfg_changed_cnt"`
	ActionsStats  struct {
		Skipped int `json:"skipped"`
	} `json:"actions_stats"`
	Relays []struct {
		Ison           bool   `json:"ison"`
		HasTimer       bool   `json:"has_timer"`
		TimerStarted   int    `json:"timer_started"`
		TimerDuration  int    `json:"timer_duration"`
		TimerRemaining int    `json:"timer_remaining"`
		Overpower      bool   `json:"overpower"`
		Source         string `json:"source"`
	} `json:"relays"`
	Meters []struct {
		Power     float64   `json:"power"`
		Overpower float64   `json:"overpower"`
		IsValid   bool      `json:"is_valid"`
		Timestamp int       `json:"timestamp"`
		Counters  []float64 `json:"counters"`
		Total     int       `json:"total"`
	} `json:"meters"`
	Temperature     float64 `json:"temperature"`
	Overtemperature bool    `json:"overtemperature"`
	Tmp             struct {
		Tc      float64 `json:"tC"`
		Tf      float64 `json:"tF"`
		IsValid bool    `json:"is_valid"`
	} `json:"tmp"`
	Update struct {
		Status     string `json:"status"`
		HasUpdate  bool   `json:"has_update"`
		NewVersion string `json:"new_version"`
		OldVersion string `json:"old_version"`
	} `json:"update"`
	RAMTotal int `json:"ram_total"`
	RAMFree  int `json:"ram_free"`
	FsSize   int `json:"fs_size"`
	FsFree   int `json:"fs_free"`
	Uptime   int `json:"uptime"`
}

const waitseconds int = 5

func init() {

	logrus.SetFormatter(&nested.Formatter{
		HideKeys:        false,
		TimestampFormat: time.RFC3339,
		FieldsOrder:     []string{"temperature", "watt"},
		// NoColors:        false,
	})

	configor.Load(&Config, "promplugs.yml")
	// fmt.Printf("config: %#v", Config)

	prometheus.Register(temperature)
	prometheus.Register(powermeter)
	prometheus.Register(shellyS_unixtime)
	prometheus.Register(shellyS_rssi)

	req, _ = http.NewRequest("GET", Config.Shelly.Url, nil)

	req.SetBasicAuth(Config.Shelly.User, Config.Shelly.Password)
}

func callShelly() {

	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("%s", err.Error())
	} else {

		bodyText, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			logrus.Errorf("%s", err.Error())
		} else {

			s := string(bodyText)

			json.Unmarshal([]byte(s), &dat)

			powermeter.Set(dat.Meters[0].Power)
			temperature.Set(dat.Temperature)
			shellyS_unixtime.Set(float64(dat.Unixtime))
			shellyS_unixtime.Set(float64(dat.WifiSta.Rssi))

			// logrus.Infof("Temperature %f; Watt %f", dat.Temperature, dat.Meters[0].Power)
			logrus.WithFields(logrus.Fields{
				"SP-Temperature": dat.Temperature,
				"SP-Meter-Watt":  dat.Meters[0].Power,
				"SP-RSSI":        dat.WifiSta.Rssi,
				"SP-Unixtime":    dat.Unixtime,
			}).Info("Call successful.")
		}
	}

}

func main() {
	logrus.Infof("Startup PromPlugS")

	/*
		s := gocron.NewScheduler(time.UTC)
		s.Every(5).Seconds().Do(callShelly)
	*/

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":9091", nil)

	for {
		time.Sleep(time.Duration(waitseconds) * time.Second)
		callShelly()
	}

}
