# 🕌 Athan Automation System

An automated Islamic prayer time notification system that plays audio files through Sonos speakers. The system calculates prayer times based on geographic coordinates and can manage both time-based and prayer-time-based events with individual audio folders and volume levels.

## 📋 Table of Contents

- [Features](#features)
- [How It Works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration Guide](#configuration-guide)
- [Audio File Setup](#audio-file-setup)
- [Building & Deployment](#building--deployment)
- [Usage](#usage)
- [MQTT Integration](#mqtt-integration)
- [Troubleshooting](#troubleshooting)
- [Advanced Configuration](#advanced-configuration)

---

## ✨ Features

- ✅ **Automatic Prayer Time Calculation** - Calculates all five daily prayers (Fajr, Zuhr, Asr, Maghrib, Isha) plus Sunrise
- ✅ **Flexible Event Scheduling** - Schedule events based on prayer times with offsets OR fixed clock times
- ✅ **Random Audio Selection** - Randomly selects from multiple audio files per event for variety
- ✅ **Individual Volume Control** - Set custom volume levels for each event (e.g., quieter at night)
- ✅ **Automatic Sonos Discovery** - Finds all Sonos speakers on your network via SSDP
- ✅ **Multi-Room Audio** - Plays simultaneously on all discovered Sonos devices
- ✅ **Backup IP Configuration** - Fallback to manual IPs if discovery fails
- ✅ **MQTT Integration** - Trigger events remotely via MQTT messages
- ✅ **Daily Auto-Recalculation** - Prayer times recalculate daily at 2:00 AM
- ✅ **Built-in HTTP Server** - Serves audio files to Sonos speakers
- ✅ **Playback Lock** - Prevents overlapping audio playback
- ✅ **Smart Volume Adjustment** - Automatically lowers volume during night hours (21:00-07:00)

---

## 🎯 How It Works

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Athan Automation System                   │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │   Prayer     │    │    Event     │    │     Cron     │ │
│  │ Calculation  │───▶│  Scheduler   │───▶│   Triggers   │ │
│  │   Engine     │    │              │    │              │ │
│  └──────────────┘    └──────────────┘    └──────┬───────┘ │
│                                                   │          │
│                                                   ▼          │
│                                          ┌──────────────┐   │
│  ┌──────────────┐                       │    Audio     │   │
│  │     SSDP     │                       │  Selection   │   │
│  │   Discovery  │                       │   Engine     │   │
│  └──────┬───────┘                       └──────┬───────┘   │
│         │                                      │            │
│         ▼                                      ▼            │
│  ┌──────────────┐                       ┌──────────────┐   │
│  │    Sonos     │◀──────────────────────│     HTTP     │   │
│  │   Speakers   │      SOAP/UPnP        │    Server    │   │
│  └──────────────┘                       └──────────────┘   │
│                                                              │
│  ┌──────────────┐                                          │
│  │     MQTT     │  (Optional remote triggering)            │
│  │   Listener   │                                          │
│  └──────────────┘                                          │
└─────────────────────────────────────────────────────────────┘
```

### Workflow

1. **Startup**: System loads configuration and discovers Sonos devices
2. **Prayer Calculation**: Calculates today's prayer times based on location
3. **Event Scheduling**: Creates cron jobs for each configured event
4. **Event Trigger**: At scheduled time, selects random audio file
5. **Sonos Control**: Ungrouping, volume setting, URI loading, playback
6. **Lock Management**: 60-second playback lock prevents overlaps
7. **Daily Reset**: Recalculates at 2:00 AM for next day

---

## 🔧 Prerequisites

### Hardware Requirements
- **Raspberry Pi** (Model 3B+, 4, or newer recommended)
- **Sonos Speakers** (one or more on the same network)
- **SD Card** (minimum 8GB, 16GB+ recommended)
- **Network Connection** (Ethernet or WiFi)

### Software Requirements
- **Operating System**: Raspberry Pi OS (Bookworm/Bullseye) or any Linux distribution
- **Go**: Version 1.19 or newer
- **Network**: All devices must be on the same local network for SSDP discovery

### Network Requirements
- Static IP address for Raspberry Pi (recommended)
- Multicast enabled on network (for SSDP discovery)
- Port 8080 accessible for HTTP audio serving
- Port 1400 accessible on Sonos speakers

---

## 📦 Installation

### Step 1: Install Go

```bash
# On Raspberry Pi OS
sudo apt update
sudo apt install golang-go

# Verify installation
go version
```

### Step 2: Clone or Create Project

```bash
# Create project directory
mkdir -p /opt/athan
cd /opt/athan

# Copy your main.go file here
# Create necessary directories
sudo mkdir -p /opt/athan/audio
sudo mkdir -p /var/log/athan
```

### Step 3: Initialize Go Module

```bash
cd /opt/athan
go mod init athan

# Download dependencies
go get github.com/eclipse/paho.mqtt.golang
go get github.com/hablullah/go-prayer
go get github.com/koron/go-ssdp
go get github.com/robfig/cron/v3
go get github.com/sirupsen/logrus
```

### Step 4: Set Permissions

```bash
sudo chmod -R 755 /opt/athan
sudo chmod -R 755 /var/log/athan
```

---

## ⚙️ Configuration Guide

### Creating config.json

Create a file named `config.json` in `/opt/athan/`:

```json
{
  "latitude": 48.7758,
  "longitude": 9.1829,
  "elevation": 250,
  "calculation_method": "MWL",
  "asr_convention": "Shafii",
  "time_corrections": {},
  "pi_ip": "192.168.1.100",
  "mqtt_broker": "tcp://192.168.1.50:1883",
  "backup_ips": [
    "192.168.1.101",
    "192.168.1.102"
  ],
  "events": [
    {
      "name": "Fajr Athan",
      "base": "fajr",
      "offset": 0,
      "folder": "fajr",
      "volume": 8
    },
    {
      "name": "Morning Adhkar",
      "base": "sunrise",
      "offset": 5,
      "folder": "morning_adhkar",
      "volume": 10
    },
    {
      "name": "Zuhr Athan",
      "base": "zuhr",
      "offset": 0,
      "folder": "athan",
      "volume": 15
    },
    {
      "name": "Asr Athan",
      "base": "asr",
      "offset": 0,
      "folder": "athan",
      "volume": 15
    },
    {
      "name": "Maghrib Athan",
      "base": "maghrib",
      "offset": 0,
      "folder": "athan",
      "volume": 15
    },
    {
      "name": "Evening Adhkar",
      "base": "maghrib",
      "offset": 10,
      "folder": "evening_adhkar",
      "volume": 12
    },
    {
      "name": "Isha Athan",
      "base": "isha",
      "offset": 0,
      "folder": "athan",
      "volume": 12
    },
    {
      "name": "Tahajjud Reminder",
      "base": "03:30",
      "offset": 0,
      "folder": "quran",
      "volume": 5
    }
  ]
}
```

### Configuration Parameters Explained

#### Location Settings

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `latitude` | float | Your latitude coordinate | `48.7758` (Stuttgart) |
| `longitude` | float | Your longitude coordinate | `9.1829` (Stuttgart) |
| `elevation` | float | Elevation above sea level in meters | `250` |

**How to find your coordinates:**
- Visit [latlong.net](https://www.latlong.net)
- Or use Google Maps (right-click → coordinates)

#### Prayer Calculation Settings

| Parameter | Type | Description | Options |
|-----------|------|-------------|---------|
| `calculation_method` | string | Method for calculating prayer times | Currently hardcoded to `MWL` (Muslim World League) |
| `asr_convention` | string | Asr calculation method | Currently hardcoded to `Shafii` |
| `time_corrections` | object | Manual adjustments to prayer times | `{"fajr": 2, "isha": -1}` (adds/subtracts minutes) |

**Note**: The code currently uses hardcoded values (`prayer.MWL` and `prayer.Shafii`). These config fields are placeholders for future enhancements.

#### Network Settings

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `pi_ip` | string | Static IP address of your Raspberry Pi | `"192.168.1.100"` |
| `mqtt_broker` | string | MQTT broker address (optional) | `"tcp://192.168.1.50:1883"` or `"none"` |
| `backup_ips` | array | Manual Sonos IP addresses (fallback) | `["192.168.1.101", "192.168.1.102"]` |

**Important:** 
- `pi_ip` should be the static IP of your Raspberry Pi
- This IP is used to construct audio URLs that Sonos speakers will fetch from
- Set `mqtt_broker` to `"none"` or `""` to disable MQTT

#### Event Configuration

Each event in the `events` array has these properties:

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `name` | string | Descriptive name for the event | `"Fajr Athan"` |
| `base` | string | Prayer time OR fixed time | `"fajr"` or `"07:30"` |
| `offset` | int | Minutes to add/subtract from base time | `5` (adds 5 min), `-10` (subtracts 10 min) |
| `folder` | string | Subdirectory in `/opt/athan/audio/` | `"fajr"`, `"athan"`, `"quran"` |
| `volume` | int | Volume level (0-100) | `15` for daytime, `5` for night |

**Base Time Options:**
- Prayer times: `"fajr"`, `"sunrise"`, `"zuhr"`, `"asr"`, `"maghrib"`, `"isha"`
- Fixed times: `"HH:MM"` format (e.g., `"07:30"`, `"22:00"`)

**Example Event Configurations:**

```json
// Play Fajr athan exactly at Fajr time
{
  "name": "Fajr Athan",
  "base": "fajr",
  "offset": 0,
  "folder": "fajr",
  "volume": 8
}

// Play morning adhkar 5 minutes after sunrise
{
  "name": "Morning Adhkar",
  "base": "sunrise",
  "offset": 5,
  "folder": "morning_adhkar",
  "volume": 10
}

// Play Quran at fixed time (3:30 AM)
{
  "name": "Tahajjud Reminder",
  "base": "03:30",
  "offset": 0,
  "folder": "quran",
  "volume": 5
}

// Play athan 2 minutes before Zuhr
{
  "name": "Zuhr Reminder",
  "base": "zuhr",
  "offset": -2,
  "folder": "reminder",
  "volume": 10
}
```

---

## 🎵 Audio File Setup

### Directory Structure

Create the following directory structure:

```
/opt/athan/audio/
├── fajr/
│   ├── fajr_athan_1.mp3
│   ├── fajr_athan_2.mp3
│   └── fajr_athan_3.mp3
├── athan/
│   ├── athan_makkah.mp3
│   ├── athan_madinah.mp3
│   └── athan_mishary.mp3
├── morning_adhkar/
│   ├── morning_adhkar_1.mp3
│   └── morning_adhkar_2.mp3
├── evening_adhkar/
│   └── evening_adhkar.mp3
├── quran/
│   ├── surah_mulk.mp3
│   ├── surah_kahf.mp3
│   └── ayatul_kursi.mp3
└── reminder/
    └── reminder_tone.mp3
```

### Audio File Requirements

- **Supported Formats**: MP3, WAV
- **Naming**: Any filename (spaces and special characters are automatically URL-encoded)
- **Organization**: Place related audio files in the same folder
- **Selection**: System randomly selects one file from the specified folder

### Adding Audio Files

```bash
# Create a new folder
sudo mkdir -p /opt/athan/audio/my_folder

# Copy audio files (example)
sudo cp /path/to/audio/*.mp3 /opt/athan/audio/my_folder/

# Set permissions
sudo chmod -R 755 /opt/athan/audio
```

---

## 🔨 Building & Deployment

### Method 1: Direct Build and Run

```bash
cd /opt/athan

# Build the binary
go build -o athan main.go

# Run directly
./athan
```

### Method 2: Build and Install as Service

#### Build the Binary

```bash
cd /opt/athan
go build -o athan main.go
sudo chmod +x athan
```

#### Create Systemd Service

Create `/etc/systemd/system/athan.service`:

```ini
[Unit]
Description=Athan Automation System
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/opt/athan
ExecStart=/opt/athan/athan
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

#### Enable and Start Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service (start on boot)
sudo systemctl enable athan

# Start service now
sudo systemctl start athan

# Check status
sudo systemctl status athan

# View logs
sudo journalctl -u athan -f
```

### Method 3: Cross-Compile for Raspberry Pi (from another machine)

```bash
# On your development machine (Linux/Mac/Windows)
# For Raspberry Pi (ARM architecture)

# Set environment variables
export GOOS=linux
export GOARCH=arm
export GOARM=7  # For Pi 3/4

# Build
go build -o athan-arm main.go

# Copy to Raspberry Pi
scp athan-arm pi@192.168.1.100:/opt/athan/athan
scp config.json pi@192.168.1.100:/opt/athan/

# On Raspberry Pi, set permissions
ssh pi@192.168.1.100
cd /opt/athan
chmod +x athan
```

---

## 🚀 Usage

### Starting the System

```bash
# If running as service
sudo systemctl start athan

# If running manually
cd /opt/athan
./athan
```

### Viewing Logs

```bash
# Systemd logs (real-time)
sudo journalctl -u athan -f

# Systemd logs (last 100 lines)
sudo journalctl -u athan -n 100

# If logging to file (requires code modification)
tail -f /var/log/athan/athan.log
```

### Testing Events

You can test events without waiting for scheduled times using MQTT (if configured):

```bash
# Install mosquitto clients
sudo apt install mosquitto-clients

# Test Fajr athan
mosquitto_pub -h 192.168.1.50 -t "athan/test/fajr" -m "test"

# Test morning adhkar
mosquitto_pub -h 192.168.1.50 -t "athan/test/morning" -m "test"

# Test normal athan (uses default folder)
mosquitto_pub -h 192.168.1.50 -t "athan/test/normal" -m "test"
```

### Checking Scheduled Events

The system logs all scheduled events at startup:

```
INFO[2024-02-11T14:23:45+01:00] Geplant: Fajr Athan      um 05:32 (Vol: 8)
INFO[2024-02-11T14:23:45+01:00] Geplant: Morning Adhkar  um 07:12 (Vol: 10)
INFO[2024-02-11T14:23:45+01:00] Geplant: Zuhr Athan      um 12:45 (Vol: 15)
INFO[2024-02-11T14:23:45+01:00] Geplant: Asr Athan       um 15:23 (Vol: 15)
INFO[2024-02-11T14:23:45+01:00] Geplant: Maghrib Athan   um 17:58 (Vol: 15)
INFO[2024-02-11T14:23:45+01:00] Geplant: Evening Adhkar  um 18:08 (Vol: 12)
INFO[2024-02-11T14:23:45+01:00] Geplant: Isha Athan      um 19:34 (Vol: 12)
INFO[2024-02-11T14:23:45+01:00] Geplant: Tahajjud        um 03:30 (Vol: 5)
```

### Manual Testing with curl

```bash
# Test audio serving
curl http://192.168.1.100:8080/fajr/fajr_athan_1.mp3 -I

# Should return HTTP 200 OK
```

---

## 📡 MQTT Integration

### MQTT Topics

The system subscribes to: `athan/test/#`

Supported subtopics:
- `athan/test/fajr` - Plays audio from `fajr` folder
- `athan/test/morning` - Plays audio from `morning_adhkar` folder
- `athan/test/normal` - Plays audio from root audio directory

### Setting Up MQTT Broker

#### Option 1: Install Mosquitto on Raspberry Pi

```bash
sudo apt update
sudo apt install mosquitto mosquitto-clients
sudo systemctl enable mosquitto
sudo systemctl start mosquitto
```

Configuration: Update `config.json`:
```json
{
  "mqtt_broker": "tcp://localhost:1883"
}
```

#### Option 2: Use External MQTT Broker

Popular options:
- **Home Assistant** (built-in MQTT broker)
- **CloudMQTT** (cloud-based)
- **HiveMQ** (cloud or self-hosted)

Configuration:
```json
{
  "mqtt_broker": "tcp://192.168.1.50:1883"
}
```

### Disabling MQTT

Set to empty string or "none":
```json
{
  "mqtt_broker": "none"
}
```

---

## 🔍 Troubleshooting

### Common Issues

#### 1. No Audio Playing

**Check Sonos Discovery:**
```bash
# View logs for discovered IPs
sudo journalctl -u athan | grep "IPs:"
```

Expected output:
```
Starte: fajr/fajr_athan_1.mp3 | Vol: 8 | IPs: [192.168.1.101 192.168.1.102]
```

**Solution:** If no IPs discovered, add manual IPs to `backup_ips` in config.json

#### 2. "Keine Audio-Datei gefunden!" Error

**Cause:** No audio files in the specified folder

**Check:**
```bash
ls -la /opt/athan/audio/fajr/
```

**Solution:** Add MP3 or WAV files to the folder

#### 3. Wrong Prayer Times

**Verify Coordinates:**
- Double-check latitude/longitude in config.json
- Ensure elevation is correct

**Test Calculation:**
```bash
# View scheduled times in logs
sudo journalctl -u athan | grep "Geplant:"
```

**Adjust if Needed:**
Use `time_corrections` (feature requires code modification):
```json
{
  "time_corrections": {
    "fajr": 2,    // Add 2 minutes
    "isha": -1    // Subtract 1 minute
  }
}
```

#### 4. Service Won't Start

**Check Service Status:**
```bash
sudo systemctl status athan
```

**Check for Errors:**
```bash
sudo journalctl -u athan -n 50
```

**Common Causes:**
- Missing config.json
- Invalid JSON in config
- Wrong file permissions
- Missing dependencies

**Solution:**
```bash
# Verify config exists
ls -la /opt/athan/config.json

# Validate JSON
cat /opt/athan/config.json | python3 -m json.tool

# Fix permissions
sudo chmod 755 /opt/athan/athan
sudo chmod 644 /opt/athan/config.json
```

#### 5. Sonos Not Responding

**Check Network:**
```bash
# Ping Sonos speaker
ping 192.168.1.101

# Test HTTP endpoint
curl http://192.168.1.101:1400/status -I
```

**Check Firewall:**
```bash
# On Raspberry Pi
sudo iptables -L
```

**Solution:** Ensure devices are on same network and multicast is enabled

#### 6. Volume Too Low/High

**Adjust Per Event:**
Edit `volume` in config.json (0-100 scale)

**Test Different Volumes:**
Use MQTT to test without waiting:
```bash
# Edit config, restart, then test
sudo systemctl restart athan
mosquitto_pub -h localhost -t "athan/test/fajr" -m "test"
```

---

## 🔧 Advanced Configuration

### Custom Prayer Calculation Methods

To use different calculation methods, modify the code in `calculate()` function:

```go
pCfg := prayer.Config{
    Latitude: appCfg.Latitude, 
    Longitude: appCfg.Longitude, 
    Elevation: appCfg.Elevation,
    CalculationMethod: prayer.ISNA,  // Change this
    AsrConvention: prayer.Hanafi,     // Change this
}
```

**Available Methods:**
- `prayer.MWL` - Muslim World League (default)
- `prayer.ISNA` - Islamic Society of North America
- `prayer.Egypt` - Egyptian General Authority
- `prayer.Makkah` - Umm Al-Qura University, Makkah
- `prayer.Karachi` - University of Islamic Sciences, Karachi
- `prayer.Tehran` - Institute of Geophysics, Tehran
- `prayer.Jafari` - Shia Ithna-Ashari

**Asr Conventions:**
- `prayer.Shafii` - Standard (default)
- `prayer.Hanafi` - Hanafi school

### Time Zone Configuration

Currently hardcoded to `Europe/Berlin`. To change:

```go
loc, _ := time.LoadLocation("America/New_York")  // Change this
```

**Common Time Zones:**
- `America/New_York`
- `America/Los_Angeles`
- `Europe/London`
- `Asia/Dubai`
- `Asia/Riyadh`
- `Asia/Karachi`

### Adding Custom MQTT Topics

Edit the MQTT subscriber in `main()`:

```go
client.Subscribe("athan/test/#", 0, func(c mqtt.Client, m mqtt.Message) {
    t := m.Topic()
    if strings.HasSuffix(t, "fajr") { 
        go callAudio("fajr", 0) 
    } else if strings.HasSuffix(t, "morning") { 
        go callAudio("morning_adhkar", 0) 
    } else if strings.HasSuffix(t, "evening") {  // NEW
        go callAudio("evening_adhkar", 0) 
    } else if strings.HasSuffix(t, "quran") {    // NEW
        go callAudio("quran", 0) 
    }
})
```

### Changing HTTP Server Port

Default is 8080. To change, modify:

```go
go http.ListenAndServe(":9000", http.FileServer(http.Dir(audioPath)))
```

**Remember:** Also update Sonos audio URL construction:
```go
audioURL := fmt.Sprintf("http://%s:9000/%s", appCfg.PiIP, safePath)
```

### Adjusting Playback Lock Duration

Default is 60 seconds. To change:

```go
time.Sleep(120 * time.Second)  // Change to 120 seconds
```

### Modifying Discovery Frequency

SSDP discovery runs every 15 minutes. To change:

```go
sys.AddFunc("*/30 * * * *", updateSonosIPs)  // Every 30 minutes
```

---

## 📊 System Behavior

### Startup Sequence
1. Load configuration
2. Initialize logging
3. Discover Sonos devices
4. Start HTTP server on port 8080
5. Connect to MQTT (if configured)
6. Calculate today's prayer times
7. Schedule all events
8. Start background jobs (recalculation, discovery)

### Daily Operations
- **2:00 AM**: Recalculate prayer times for new day
- **Every 15 minutes**: Refresh Sonos device discovery
- **At event time**: Select random audio, play on all speakers
- **After playback**: 60-second lock prevents overlaps

### Volume Logic
If event volume is 0 or not set:
- **Night (21:00-07:00)**: Volume 5
- **Day (07:00-21:00)**: Volume 15

If event volume is set: Uses that value regardless of time

### Sonos Control Flow
1. Ungroup speaker from any existing group
2. Set volume
3. Load audio URI
4. Start playback

---

## 🔐 Security Considerations

### Network Security
- Keep Raspberry Pi on private network
- Use firewall rules if exposing port 8080
- Consider VPN for remote MQTT access

### File Permissions
```bash
# Recommended permissions
sudo chown -R pi:pi /opt/athan
sudo chmod 755 /opt/athan
sudo chmod 644 /opt/athan/config.json
sudo chmod 755 /opt/athan/athan
sudo chmod -R 755 /opt/athan/audio
```

### MQTT Security
For production, use authentication:

```go
opts := mqtt.NewClientOptions().
    AddBroker(appCfg.MqttBroker).
    SetClientID("athan_pi").
    SetUsername("your_username").
    SetPassword("your_password")
```

---

## 🤝 Contributing

This is a personal project, but suggestions are welcome:
- Report bugs via issues
- Suggest features
- Submit pull requests
- Share your configuration examples

---

## 📝 License

This project is provided as-is for personal use. Modify and distribute freely.

---

## 🙏 Acknowledgments

- **go-prayer**: Prayer time calculations
- **paho.mqtt.golang**: MQTT client
- **go-ssdp**: Sonos device discovery
- **cron**: Event scheduling
- **logrus**: Logging

---

## 📞 Support

For issues or questions:
1. Check the [Troubleshooting](#troubleshooting) section
2. Review logs: `sudo journalctl -u athan -f`
3. Verify configuration: `cat /opt/athan/config.json | python3 -m json.tool`
4. Test connectivity: `ping <sonos_ip>` and `curl http://<pi_ip>:8080/`

---

## 🔄 Version History

### Current Version
- Multi-room Sonos support
- Flexible event scheduling (prayer times + fixed times)
- Individual event volumes
- MQTT integration
- Automatic daily recalculation
- SSDP discovery with backup IPs

---

**May this system help you remember and observe your prayers on time. Allahu Akbar! 🕋**
=======
# athan-sonos
Automatic Athan / audio player for Sonos speakers. Runs in the background.
>>>>>>> origin/main
