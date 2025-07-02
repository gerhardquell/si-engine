# SIGO - SI Gateway in GO

Eine minimalistische, UNIX-konforme Schnittstelle zu verschiedenen SI/KI-Anbietern.

## Features

- **KISS-Prinzip**: Eine Datei, keine Dependencies
- **Multi-Provider**: Claude, GPT-4, DeepSeek, lokale Modelle
- **Sessions**: Persistente Gespräche  
- **UNIX-konform**: stdin/stdout, pipe-fähig
- **Robust**: Circuit Breaker, Retries
- **Erweiterbar**: Einfache JSON-Configs für neue Provider

## Installation

```bash
# Kompilieren
go build -o sigo sigoEngine.go

# Oder mit GCC
gccgo -o sigo sigoEngine.go

# Installieren
sudo cp sigo /usr/local/bin/
```

## Konfiguration

Erstelle eine `.modelname.config` Datei:

```json
{
  "endpoint": "https://api.anthropic.com/v1/messages",
  "model": "claude-3-5-sonnet-20241022",
  "api_key": "${ANTHROPIC_API_KEY}",
  "type": "anthropic"
}
```

Unterstützte Typen: `anthropic`, `openai`

### Beispiel-Configs

**.claude4.config**
```json
{
  "endpoint": "https://api.anthropic.com/v1/messages",
  "model": "claude-3-5-sonnet-20241022", 
  "api_key": "${ANTHROPIC_API_KEY}",
  "type": "anthropic"
}
```

**.gpt4.config**
```json
{
  "endpoint": "https://api.openai.com/v1/chat/completions",
  "model": "gpt-4",
  "api_key": "${OPENAI_API_KEY}",
  "type": "openai"
}
```

**.deepseek.config**
```json
{
  "endpoint": "https://api.deepseek.com/v1/chat/completions",
  "model": "deepseek-coder",
  "api_key": "${DEEPSEEK_API_KEY}",
  "type": "openai"
}
```

## Verwendung

### Einfache Anfragen

```bash
# Standard (claude4)
sigo "Erkläre Quantenphysik"

# Anderes Modell
sigo -m gpt4 "Hello GPT"

# Mit mehr Tokens
sigo -n 2000 "Schreibe eine Geschichte"
```

### Sessions (Gespräche)

```bash
# Neue Session starten
sigo -s projekt "Lass uns über KI-Ethik reden"

# Session fortsetzen  
sigo -s projekt "Was meinst du zu Asimovs Gesetzen?"

# Andere SI, andere Session
sigo -m deepseek -s code "Hilf mir bei Python"
```

### UNIX Pipes

```bash
# Von Datei
cat frage.txt | sigo

# In Datei
sigo "Generiere README" > README.md

# Pipeline
echo "Fasse zusammen:" | sigo -s review < artikel.txt

# Mit anderen Tools
sigo "SQL für User-Tabelle" | psql mydb
```

### Weitere Optionen

```bash
# Hilfe und Übersicht
sigo -h

# Quiet Mode (nur Antwort)
sigo -q "Quick answer"

# JSON Output
sigo -j "Test" | jq .response

# Timeout und Retries
sigo -t 60 -r 5 "Komplexe Frage"
```

## Sessions

Sessions werden in `.sessions/` gespeichert:

```
.sessions/
├── claude4-projekt.json
├── gpt4-analyse.json
└── deepseek-code.json
```

Die JSON-Dateien können mit jedem Editor bearbeitet werden.

## Fehlerbehandlung

- Circuit Breaker nach 3 Fehlern
- Automatische Retries mit Backoff
- Klare Fehlermeldungen auf stderr

## Map-Reduce Erweiterung

Für komplexe Aufgaben gibt es Wrapper:

```bash
# Mehrere SIs parallel fragen
sigo-mapreduce "Komplexe Frage" \
  --map "claude4,gpt4,deepseek" \
  --reduce "claude4"
```

## Philosophie

SIGO folgt der UNIX-Philosophie:
- Do one thing well
- Text streams als universelle Schnittstelle  
- Compose-fähig mit anderen Tools
- Einfachheit über Features

## Lizenz

Copyright 2025 Gerhard Quell - SKEQuell

## Credits

Entwickelt im Geiste von:
- A.E. van Vogt's Nexialismus
- UNIX Philosophy
- KISS-Prinzip

"Eine Brücke zwischen Biologischer und Synthetischer Intelligenz"
