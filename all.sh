# 1. Zielverzeichnis für Backups festlegen
BACKUP_DIR="./notNormalized"

# 2. Verzeichnisstruktur im Backup-Ordner spiegeln
find . -type d -exec mkdir -p "$BACKUP_DIR/{}" \;

# 3. Originale verschieben (Backup erstellen)
find . -type f -name "*.mp3" -exec mv {} "$BACKUP_DIR/{}" \;

# 4. Dateien vom Backup-Ordner zurück in den aktuellen Ordner normalisieren
find "$BACKUP_DIR" -type f -name "*.mp3" | while read -r file; do
    # Zielpfad berechnen (relativ zum aktuellen Verzeichnis)
    target="./${file#$BACKUP_DIR/}"
    
    echo "Verarbeite: $file -> $target"
    ffmpeg -i "$file" -filter:a loudnorm "$target" -loglevel error
done