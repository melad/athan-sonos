package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hablullah/go-prayer"
	"github.com/koron/go-ssdp"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// --- Strukturen ---

type EventConfig struct {
	Name   string `json:"name"`
	Base   string `json:"base"`
	Offset int    `json:"offset"`
	Folder string `json:"folder"`
	Volume int    `json:"volume"`
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

type SonosState struct {
	TrackURI       string
	TrackMetaData  string
	RelTime        string
	PlayMode       string
	TransportState string
	Volume         int
	TrackNumber    int
}

var (
	log           = logrus.New()
	audioPath     = "/opt/athan/audio"
	appCfg        AppConfig
	client        mqtt.Client
	mainCron      = cron.New()
	athanRunning  = false
	muAthan       sync.Mutex
	discoveredIPs []string
	ipMutex       sync.RWMutex
)

// --- Hilfsfunktionen ---

func loadConfig() {
	file, err := os.Open("config.json")
	if err != nil {
		log.Fatalf("Fehler config.json: %v", err)
	}
	defer file.Close()
	json.NewDecoder(file).Decode(&appCfg)
}

func getRandomAthan(subDir string) string {
	searchDir := filepath.Join(audioPath, subDir)
	files, err := os.ReadDir(searchDir)
	if err != nil || len(files) == 0 {
		return ""
	}
	var playable []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if !f.IsDir() && (ext == ".mp3" || ext == ".wav") {
			playable = append(playable, f.Name())
		}
	}
	if len(playable) == 0 {
		return ""
	}
	return filepath.Join(subDir, playable[rand.Intn(len(playable))])
}

// --- SOAP Helfer ---

func soap(ctx context.Context, ip, service, action, body string) error {
	endpoint := fmt.Sprintf("http://%s:1400/MediaRenderer/%s/Control", ip, service)
	env := fmt.Sprintf(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>%s</s:Body></s:Envelope>`, body)
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(env))
	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Set("SOAPACTION", fmt.Sprintf(`"urn:schemas-upnp-org:service:%s:1#%s"`, service, action))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func soapGet(ctx context.Context, ip, service, action, body string) (string, error) {
	endpoint := fmt.Sprintf("http://%s:1400/MediaRenderer/%s/Control", ip, service)
	env := fmt.Sprintf(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>%s</s:Body></s:Envelope>`, body)
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(env))
	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Set("SOAPACTION", fmt.Sprintf(`"urn:schemas-upnp-org:service:%s:1#%s"`, service, action))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	io.Copy(buf, resp.Body)
	return buf.String(), nil
}

func extractXML(body, tag string) string {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(body, open)
	end := strings.Index(body, close)
	if start == -1 || end == -1 {
		return ""
	}
	return body[start+len(open) : end]
}

// --- Restore Logik ---

func getSonosState(ctx context.Context, ip string) (*SonosState, error) {
	posResp, _ := soapGet(ctx, ip, "AVTransport", "GetPositionInfo", `<u:GetPositionInfo xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID></u:GetPositionInfo>`)
	transResp, _ := soapGet(ctx, ip, "AVTransport", "GetTransportInfo", `<u:GetTransportInfo xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID></u:GetTransportInfo>`)
	settingsResp, _ := soapGet(ctx, ip, "AVTransport", "GetTransportSettings", `<u:GetTransportSettings xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID></u:GetTransportSettings>`)
	volResp, _ := soapGet(ctx, ip, "RenderingControl", "GetVolume", `<u:GetVolume xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"><InstanceID>0</InstanceID><Channel>Master</Channel></u:GetVolume>`)

	state := &SonosState{
		TrackURI:       extractXML(posResp, "TrackURI"),
		TrackMetaData:  extractXML(posResp, "TrackMetaData"),
		RelTime:        extractXML(posResp, "RelTime"),
		TransportState: extractXML(transResp, "CurrentTransportState"),
		PlayMode:       extractXML(settingsResp, "PlayMode"),
	}
	fmt.Sscanf(extractXML(volResp, "CurrentVolume"), "%d", &state.Volume)
	fmt.Sscanf(extractXML(posResp, "Track"), "%d", &state.TrackNumber)
	return state, nil
}

func restoreSonosState(ctx context.Context, ip string, state *SonosState) {
	if state == nil || state.TrackURI == "" || strings.Contains(state.TrackURI, "8080") {
		log.Warnf("[%s] Restore übersprungen (ungültige URI).", ip)
		return
	}

	log.Infof("[%s] Wiederherstellung startet...", ip)

	// 1. Lautstärke zurück
	_ = soap(ctx, ip, "RenderingControl", "SetVolume", fmt.Sprintf(`<u:SetVolume xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"><InstanceID>0</InstanceID><Channel>Master</Channel><DesiredVolume>%d</DesiredVolume></u:SetVolume>`, state.Volume))

	// 2. Die ursprüngliche URI setzen (Das lädt entweder den Stream ODER die Queue)
	err := soap(ctx, ip, "AVTransport", "SetAVTransportURI", fmt.Sprintf(`<u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><CurrentURI>%s</CurrentURI><CurrentURIMetaData>%s</CurrentURIMetaData></u:SetAVTransportURI>`, htmlEscape(state.TrackURI), htmlEscape(state.TrackMetaData)))
	if err != nil {
		log.Errorf("[%s] Fehler beim Wiederherstellen der URI: %v", ip, err)
	}

	// 3. Wenn es eine Playlist war, zum Track springen
	if state.TrackNumber > 1 {
		_ = soap(ctx, ip, "AVTransport", "Seek", fmt.Sprintf(`<u:Seek xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><Unit>TRACK_NR</Unit><Target>%d</Target></u:Seek>`, state.TrackNumber))
	}

	// 4. PlayMode & Zeit-Seek
	_ = soap(ctx, ip, "AVTransport", "SetPlayMode", fmt.Sprintf(`<u:SetPlayMode xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><NewPlayMode>%s</NewPlayMode></u:SetPlayMode>`, state.PlayMode))

	if state.RelTime != "" && state.RelTime != "NOT_IMPLEMENTED" && !strings.Contains(state.TrackURI, "stream") {
		_ = soap(ctx, ip, "AVTransport", "Seek", fmt.Sprintf(`<u:Seek xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><Unit>REL_TIME</Unit><Target>%s</Target></u:Seek>`, state.RelTime))
	}

	// 5. Playback starten
	if state.TransportState == "PLAYING" {
		_ = soap(ctx, ip, "AVTransport", "Play", `<u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><Speed>1</Speed></u:Play>`)
	}
}

func waitForAthanEnd(ip string, maxWait time.Duration) {
	deadline := time.Now().Add(maxWait)
	hasStarted := false
	time.Sleep(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		resp, err := soapGet(ctx, ip, "AVTransport", "GetTransportInfo", `<u:GetTransportInfo xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID></u:GetTransportInfo>`)
		cancel()
		if err == nil {
			state := extractXML(resp, "CurrentTransportState")
			if state == "PLAYING" {
				hasStarted = true
			}
			if hasStarted && (state == "STOPPED" || state == "PAUSED_PLAYBACK") {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
}

// --- Main Logik ---

func callAudio(subDir string, overrideVol int) {
	muAthan.Lock()
	if athanRunning {
		muAthan.Unlock()
		return
	}
	athanRunning = true
	muAthan.Unlock()
	defer func() { muAthan.Lock(); athanRunning = false; muAthan.Unlock() }()

	fileName := getRandomAthan(subDir)
	if fileName == "" {
		return
	}

	parts := strings.Split(fileName, string(os.PathSeparator))
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	audioURL := fmt.Sprintf("http://%s:8080/%s", appCfg.PiIP, strings.Join(parts, "/"))

	vol := overrideVol
	if vol <= 0 {
		h := time.Now().Hour()
		vol = 20
		if h >= 21 || h < 7 {
			vol = 8
		}
	}

	ipMutex.RLock()
	ips := append([]string{}, discoveredIPs...)
	if len(ips) == 0 {
		ips = appCfg.BackupIPs
	}
	ipMutex.RUnlock()

	var wg sync.WaitGroup
	for _, ip := range ips {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()
			ctx := context.Background()

			// 1. Sichern
			state, _ := getSonosState(ctx, targetIP)

			// 2. Vorbereiten
			_ = soap(ctx, targetIP, "AVTransport", "BecomeCoordinatorOfStandaloneGroup", `<u:BecomeCoordinatorOfStandaloneGroup xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID></u:BecomeCoordinatorOfStandaloneGroup>`)
			time.Sleep(1 * time.Second)
			_ = soap(ctx, targetIP, "RenderingControl", "SetVolume", fmt.Sprintf(`<u:SetVolume xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"><InstanceID>0</InstanceID><Channel>Master</Channel><DesiredVolume>%d</DesiredVolume></u:SetVolume>`, vol))

			// 3. Play
			meta := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="1" parentID="0" restricted="1"><dc:title>Athan</dc:title><upnp:class>object.item.audioItem.musicTrack</upnp:class><res protocolInfo="http-get:*:audio/mpeg:*">` + audioURL + `</res></item></DIDL-Lite>`
			if err := soap(ctx, targetIP, "AVTransport", "SetAVTransportURI", fmt.Sprintf(`<u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><CurrentURI>%s</CurrentURI><CurrentURIMetaData>%s</CurrentURIMetaData></u:SetAVTransportURI>`, audioURL, htmlEscape(meta))); err == nil {
				_ = soap(ctx, targetIP, "AVTransport", "SetPlayMode", `<u:SetPlayMode xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><NewPlayMode>NORMAL</NewPlayMode></u:SetPlayMode>`)
				_ = soap(ctx, targetIP, "AVTransport", "Play", `<u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><InstanceID>0</InstanceID><Speed>1</Speed></u:Play>`)
				waitForAthanEnd(targetIP, 8*time.Minute)
			}

			// 4. Restore
			restoreSonosState(ctx, targetIP, state)
		}(ip)
	}
	wg.Wait()
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// --- Cron & Service ---

func calculate() {
	mainCron.Stop()
	mainCron = cron.New()
	loadConfig()
	loc, _ := time.LoadLocation("Europe/Berlin")
	pCfg := prayer.Config{Latitude: appCfg.Latitude, Longitude: appCfg.Longitude, Elevation: appCfg.Elevation, CalculationMethod: prayer.MWL, AsrConvention: prayer.Shafii}
	res, _ := prayer.Calculate(pCfg, time.Now().In(loc))
	tm := map[string]time.Time{"fajr": res.Fajr, "sunrise": res.Sunrise, "zuhr": res.Zuhr, "asr": res.Asr, "maghrib": res.Maghrib, "isha": res.Isha}

	for _, ev := range appCfg.Events {
		var target time.Time
		if strings.Contains(ev.Base, ":") {
			t, _ := time.ParseInLocation("15:04", ev.Base, loc)
			target = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), t.Hour(), t.Minute(), 0, 0, loc)
		} else {
			base, ok := tm[strings.ToLower(ev.Base)]
			if !ok {
				continue
			}
			target = base.Add(time.Duration(ev.Offset) * time.Minute)
		}
		if target.Before(time.Now()) {
			continue
		}
		h, m, _ := target.Clock()
		mainCron.AddFunc(fmt.Sprintf("%d %d * * *", m, h), func() { go callAudio(ev.Folder, ev.Volume) })
		log.Infof("Geplant: %s um %02d:%02d", ev.Name, h, m)
	}
	mainCron.Start()
}

func updateSonosIPs() {
	list, err := ssdp.Search("urn:schemas-upnp-org:device:ZonePlayer:1", 3, "")
	if err != nil {
		return
	}
	var newIPs []string
	found := make(map[string]bool)
	for _, r := range list {
		ip := strings.Split(strings.TrimPrefix(r.Location, "http://"), ":")[0]
		if ip != "" && !found[ip] {
			newIPs = append(newIPs, ip)
			found[ip] = true
		}
	}
	if len(newIPs) > 0 {
		ipMutex.Lock()
		discoveredIPs = newIPs
		ipMutex.Unlock()
	}
}

func main() {
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	log.SetLevel(logrus.DebugLevel)
	loadConfig()
	updateSonosIPs()
	go http.ListenAndServe(":8080", http.FileServer(http.Dir(audioPath)))

	if appCfg.MqttBroker != "" && appCfg.MqttBroker != "none" {
		opts := mqtt.NewClientOptions().AddBroker(appCfg.MqttBroker).SetClientID("athan_pi")
		client = mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() == nil {
			client.Subscribe("athan/test/#", 0, func(c mqtt.Client, m mqtt.Message) {
				if strings.HasSuffix(m.Topic(), "fajr") {
					go callAudio("fajr", 0)
				}
				if strings.HasSuffix(m.Topic(), "normal") {
					go callAudio("", 0)
				}
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
