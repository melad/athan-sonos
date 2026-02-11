package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/hablullah/go-prayer"
	"github.com/koron/go-ssdp"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

type EventConfig struct {
	Name   string `json:"name"`
	Base   string `json:"base"`   // "fajr" oder "07:00"
	Offset int    `json:"offset"`
	Folder string `json:"folder"`
	Volume int    `json:"volume"` // Individuelle Lautstärke
}

type AppConfig struct {
	Latitude          float64        `json:"latitude"`
	Longitude         float64        `json:"longitude"`
	Elevation         float64        `json:"elevation"`
	CalculationMethod string         `json:"calculation_method"`
	AsrConvention     string         `json:"asr_convention"`
	TimeCorrections   map[string]int `json:"time_corrections"`
	PiIP              string         `json:"pi_ip"`
	MqttBroker        string         `json:"mqtt_broker"`
	BackupIPs         []string       `json:"backup_ips"`
	Events            []EventConfig  `json:"events"`
}

var (
	log          = logrus.New()
	logBaseLog   = "/var/log/athan/"
	audioPath    = "/opt/athan/audio"
	appCfg       AppConfig
	client       mqtt.Client
	mainCron     = cron.New()
	athanRunning = false
	muAthan      sync.Mutex
	discoveredIPs []string
	ipMutex       sync.RWMutex
)

// --- Hilfsfunktionen ---

func loadConfig() {
	file, err := os.Open("config.json")
	if err != nil { log.Fatalf("Fehler beim Laden der config.json: %v", err) }
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&appCfg); err != nil { log.Fatalf("JSON Fehler: %v", err) }
}

func getRandomAthan(subDir string) string {
	searchDir := filepath.Join(audioPath, subDir)
	files, err := os.ReadDir(searchDir)
	if err != nil || len(files) == 0 { return "" }
	var playable []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if !f.IsDir() && (ext == ".mp3" || ext == ".wav") { playable = append(playable, f.Name()) }
	}
	if len(playable) == 0 { return "" }
	return filepath.Join(subDir, playable[rand.Intn(len(playable))])
}

// --- Sonos Logik ---

func callAudio(subDir string, overrideVol int) {
	muAthan.Lock()
	if athanRunning {
		muAthan.Unlock()
		log.Warn("Wiedergabe übersprungen: System gesperrt.")
		return
	}
	athanRunning = true
	muAthan.Unlock()

	go func() {
		defer func() {
			time.Sleep(60 * time.Second)
			muAthan.Lock()
			athanRunning = false
			muAthan.Unlock()
		}()

		fileName := getRandomAthan(subDir)
		if fileName == "" { 
			log.Error("Keine Audio-Datei gefunden!")
			return 
		}

		// URL Encoding für Sonderzeichen/Leerzeichen
		pathParts := strings.Split(fileName, "/")
		for i, p := range pathParts { pathParts[i] = url.PathEscape(p) }
		safePath := strings.Join(pathParts, "/")

		ipMutex.RLock()
		ips := discoveredIPs
		if len(ips) == 0 { ips = appCfg.BackupIPs }
		ipMutex.RUnlock()

		audioURL := fmt.Sprintf("http://%s:8080/%s", appCfg.PiIP, safePath)
		
		// Lautstärke bestimmen: Event-Vol > Standard-Vol
		vol := overrideVol
		if vol <= 0 {
			now := time.Now()
			if now.Hour() >= 21 || now.Hour() < 7 { vol = 5 } else { vol = 15 }
		}

		log.Infof("Starte: %s | Vol: %d | IPs: %v", fileName, vol, ips)

		var wg sync.WaitGroup
		for _, ip := range ips {
			wg.Add(1)
			go func(targetIP string) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				_ = soap(ctx, targetIP, "AVTransport", "BecomeCoordinatorOfStandaloneGroup", `<u:BecomeCoordinatorOfStandaloneGroup xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID></u:BecomeCoordinatorOfStandaloneGroup>`)
				vXML := fmt.Sprintf(`<u:SetVolume xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"><InstanceID>0</InstanceID><Channel>Master</Channel><DesiredVolume>%d</DesiredVolume></u:SetVolume>`, vol)
				_ = soap(ctx, targetIP, "RenderingControl", "SetVolume", vXML)
				uXML := fmt.Sprintf(`<u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><CurrentURI>%s</CurrentURI><CurrentURIMetaData></CurrentURIMetaData></u:SetAVTransportURI>`, audioURL)
				_ = soap(ctx, targetIP, "AVTransport", "SetAVTransportURI", uXML)
				_ = soap(ctx, targetIP, "AVTransport", "Play", `<u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><Speed>1</Speed></u:Play>`)
			}(ip)
		}
		wg.Wait()
	}()
}

func soap(ctx context.Context, ip, service, action, body string) error {
	url := fmt.Sprintf("http://%s:1400/MediaRenderer/%s/Control", ip, service)
	env := fmt.Sprintf(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>%s</s:Body></s:Envelope>`, body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(env))
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("SOAPACTION", fmt.Sprintf("\"urn:schemas-upnp-org:service:%s:1#%s\"", service, action))
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return err }
	resp.Body.Close()
	return nil
}

// --- Planungs-Logik ---

func calculate() {
	mainCron.Stop()
	mainCron = cron.New()
	loadConfig()

	loc, _ := time.LoadLocation("Europe/Berlin")
	pCfg := prayer.Config{
		Latitude: appCfg.Latitude, Longitude: appCfg.Longitude, Elevation: appCfg.Elevation,
		CalculationMethod: prayer.MWL, AsrConvention: prayer.Shafii,
	}
	res, _ := prayer.Calculate(pCfg, time.Now().In(loc))
	timeMap := map[string]time.Time{
		"fajr": res.Fajr, "sunrise": res.Sunrise, "zuhr": res.Zuhr, "asr": res.Asr, "maghrib": res.Maghrib, "isha": res.Isha,
	}

	for _, ev := range appCfg.Events {
		var target time.Time
		if strings.Contains(ev.Base, ":") { // Feste Zeit
			t, err := time.ParseInLocation("15:04", ev.Base, loc)
			if err == nil {
				now := time.Now().In(loc)
				target = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, loc)
			}
		} else { // Gebetszeit
			base, exists := timeMap[strings.ToLower(ev.Base)]
			if !exists { continue }
			target = base.Add(time.Duration(ev.Offset) * time.Minute)
		}

		h, m, _ := target.Clock()
		evFolder := ev.Folder
		evVol := ev.Volume
		mainCron.AddFunc(fmt.Sprintf("%d %d * * *", m, h), func() { go callAudio(evFolder, evVol) })
		log.Infof("Geplant: %-15s um %02d:%02d (Vol: %d)", ev.Name, h, m, evVol)
	}
	mainCron.Start()
}

func updateSonosIPs() {
	list, err := ssdp.Search("urn:schemas-upnp-org:device:ZonePlayer:1", 3, "")
	if err != nil { return }
	var newIPs []string
	found := make(map[string]bool)
	for _, r := range list {
		ip := strings.Split(strings.TrimPrefix(r.Location, "http://"), ":")[0]
		if ip != "" && !found[ip] { newIPs = append(newIPs, ip); found[ip] = true }
	}
	if len(newIPs) > 0 { ipMutex.Lock(); discoveredIPs = newIPs; ipMutex.Unlock() }
}

func main() {
	log.Out = os.Stdout // Für Live-Debugging im Terminal
	log.Formatter = &logrus.TextFormatter{FullTimestamp: true}

	loadConfig()
	updateSonosIPs()
	go http.ListenAndServe(":8080", http.FileServer(http.Dir(audioPath)))

	if appCfg.MqttBroker != "" && appCfg.MqttBroker != "none" {
		opts := mqtt.NewClientOptions().AddBroker(appCfg.MqttBroker).SetClientID("athan_pi")
		client = mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() == nil {
			client.Subscribe("athan/test/#", 0, func(c mqtt.Client, m mqtt.Message) {
				t := m.Topic()
				if strings.HasSuffix(t, "fajr") { go callAudio("fajr", 0) } else
				if strings.HasSuffix(t, "morning") { go callAudio("morning_adhkar", 0) } else
				if strings.HasSuffix(t, "normal") { go callAudio("", 0) }
			})
		}
	}

	calculate()
	
	sys := cron.New()
	sys.AddFunc("0 2 * * *", calculate)
	sys.AddFunc("*/15 * * * *", updateSonosIPs)
	sys.Start()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}