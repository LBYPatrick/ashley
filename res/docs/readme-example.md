# MyProject - Weather Station API

A mock weather station backend providing real-time sensor data, compliant with the OpenWeather Data Collector specification.

## Features

- Full HTTP API implementation with all required endpoints
- WebSocket streaming for state and sensor data
- Session recording with proper folder structure and metadata
- Docker containerization support
- Simulated sensors: 3 thermometers and 1 barometric pressure sensor

## Tech Stack

- **Python 3.13+** (managed via `uv`)
- **FastAPI** - Web framework
- **Uvicorn** - ASGI server
- **aiofiles** - Async file operations
- **aiohttp** - Async HTTP client
- **numpy** - Numerical operations
- **uvloop** - High-performance event loop

## API Endpoints

### HTTP Endpoints

- `GET /state` - Get current station state
- `POST /session/start` - Start recording a session
- `POST /session/stop` - Stop recording and save session data
- `GET /sensors` - List available sensors
- `POST /station/start` - Start the station
- `POST /station/stop` - Stop the station
- `POST /station/maintenance` - Enter maintenance mode
- `DELETE /station/maintenance` - Exit maintenance mode
- `GET /health` - Health check

### WebSocket Endpoints

- `WS /ws/state` - Stream station state (~30 messages/sec)
- `WS /ws/sensors/stream/<device_id>` - Stream sensor data
  - `thermo_indoor`, `thermo_outdoor`, `thermo_ground` - Temperature streams
  - `barometer` - Float pressure values in hPa

## Sensors

The weather station provides the following sensors:

1. **thermo_indoor** - Indoor temperature stream (Celsius)
2. **thermo_outdoor** - Outdoor temperature stream (Celsius)
3. **thermo_ground** - Ground temperature stream (Celsius)
4. **barometer** - Barometric pressure in hPa

## Station States

The station supports the following states:

- `OFF` - Station is not active
- `BOOT` - Station is booting
- `ACTIVE` - Ready for data collection
- `CALIBRATE` - Station is calibrating sensors
- `RECORD` - Station is recording a session
- `STOPPED` - Station has stopped operation
- `PAUSE` - Recording paused, can resume
- `MAINTENANCE` - Station is in maintenance mode

## Session Data Format

Sessions are saved to `/data/session_root` with the following structure:

```
YYYYMMDD_HHMMSS_NNNNNN/
├── session-metadata.json
├── data.parquet
└── .state.json
```

The `.state.json` file is created when the session is ready for processing.

## Docker Setup

### Build the Docker Image

```bash
docker build -t myproject:latest .
```

### Run the Container

```bash
docker run -d \
  -p 8080:8080 \
  -v /path/to/session_root:/data/session_root:rw \
  -v /path/to/scratch_space:/data/scratch_space:rw \
  -v /path/to/config.jsonc:/data/config.jsonc:ro \
  myproject:latest
```

### Docker Volumes

- `/data/session_root` - Session data storage (read-write)
- `/data/scratch_space` - Temporary/intermediate data (read-write, may be volatile)
- `/data/config.jsonc` - Station configuration (read-only)

## Development

### Prerequisites

- Python 3.13+
- [uv](https://github.com/astral-sh/uv) package manager

### Installation

```bash
# Install dependencies
uv pip install -e .

# Run the application
python -m myproject.main
# or
uvicorn myproject.app:app --host 0.0.0.0 --port 8080
```

### Testing

The API will be available at `http://localhost:8080`

Example requests:

```bash
# Get station state
curl http://localhost:8080/state

# Start station
curl -X POST http://localhost:8080/station/start

# Start recording
curl -X POST http://localhost:8080/session/start

# Stop recording
curl -X POST http://localhost:8080/session/stop \
  -H "Content-Type: application/json" \
  -d '{"is_success": true}'

# Get sensors
curl http://localhost:8080/sensors
```

## Configuration

See `config.jsonc` for an example configuration file. The configuration is read from `/data/config.jsonc` when running in Docker.

## License

MIT License
