# 🔊 Sonos Athan & Adhkar Scheduler

A highly flexible, production-ready Prayer Call (Athan) and Adhkar scheduler for Sonos systems. This application calculates daily prayer times based on your geographical coordinates and plays random audio files from specified folders on your Sonos speakers.



## ✨ Features

* **Random Selection:** Automatically picks a random file from a subfolder (e.g., to hear different Muadhins).
* **Fully Flexible Event Logic:** Configure any number of events based on prayer times (e.g., "30 min before Fajr" or "30 min before Sunrise/Fajr_End").
* **Sonos Auto-Discovery:** Automatically detects speakers in your network via SSDP/UPnP.
* **MQTT Integration:** Test interface to trigger audio instantly and publish prayer time updates to home automation systems.
* **Production-Grade:** Features log rotation, graceful shutdown, and panic protection (error recovery).

---

## 📂 Project Structure

The application expects the following structure in its working directory (e.g., `athan/`):

```text
.
├── athan            # The compiled binary
├── config.json      # Configuration file
└── audio/           # Main folder for standard Athans (.mp3/.wav)
    ├── fajr/        # Specific Athans for Fajr time
    ├── morning_adhkar/  # Morning prayers / Adhkar
    └── evening_adhkar/  # Evening prayers / Adhkar