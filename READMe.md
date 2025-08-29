# Cremation Furnace API

A production-ready Go service for controlling and monitoring a cremation furnace.  
It implements RESTful endpoints, WebSocket streaming for real-time state updates, structured logging, and SQLite persistence.  
The project follows clean architecture principles and idiomatic Go best practices.

---

## 📖 Overview

This application provides an API interface to manage the cremation process, including furnace operating modes, real-time monitoring, and event logging.

It is designed with scalability, clarity, and maintainability in mind, and all major functionality is covered by unit and integration tests.

---

## 🔧 Functional Capabilities

### 1. Furnace Operating Modes
- Start and stop the furnace.
- Set target temperature and working time in **Heating** mode.

**Supported modes:**
- Heating to a specified temperature
- Cooling
- Standby (waiting mode)

### 2. State Monitoring
Retrieve the current furnace state:
- Current temperature
- Current operating mode
- Remaining work time (if applicable)
- Error notifications (overheating, sensor failure, etc.)

### 3. Logging
- All operations are logged (start/stop, mode changes, errors).
- Access to the event history with filtering by date and type.

### 4. Additional Features
- Real-time updates over **WebSocket**.
- **JWT-based authentication** for API security.
- Designed with future scalability in mind.

---

## 🛠️ Tech Stack

- **Language:** Go 1.24+
- **Framework:** Gin
- **Database:** SQLite (pure Go driver `modernc.org/sqlite`)
- **API Docs:** Swagger (OpenAPI)
- **Logging:** Zap
- **Auth:** JWT
- **Real-time:** WebSocket

---

## ▶️ Running Locally

```bash
go mod tidy
go run ./cmd/main.go
```

Server starts at:  
<http://localhost:8080>

---

## 🐳 Running with Docker

### Build

```bash
docker build -t furnace .
```

### Run

```bash
docker run --rm -p 8080:8080 --name furnace furnace
```

### Persist Database

```bash
mkdir -p ./data
docker run --rm -p 8080:8080 -v $(pwd)/data:/app --name furnace furnace
```

---

## 📖 API Documentation

Interactive Swagger UI is available at:  
👉 <http://localhost:8080/swagger/index.html>

---

## 🧪 Testing

```bash
go test ./... -v
```

---

## 📂 Project Structure

```
cmd/
  main.go          # entrypoint
internal/
  handlers/        # HTTP handlers, middleware, WebSocket
  models/          # data models
  repository/      # database access (SQLite)
  service/         # business logic
configs/
  config.yaml      # default configuration
```




