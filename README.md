# Tic-Tac-Toe Nakama Backend

This project implements a Tic-Tac-Toe game backend using Nakama, a distributed server for social and realtime games and apps. The backend is written in Go as a Nakama module/plugin.

## Project Structure

- `modules/main.go` - Main module initialization and RPC handlers
- `modules/match.go` - Game match logic for Tic-Tac-Toe
- `modules/go.mod` - Go module dependencies
- `docker-compose.yml` - Local development setup with Nakama and PostgreSQL

## Local Development

### Prerequisites

- Docker and Docker Compose
- Go 1.22+ (for building the plugin)

### Running Locally

1. Clone the repository and navigate to the project directory.

2. Start the services using Docker Compose:
   ```bash
   docker-compose up -d
   ```

   This will start:
   - PostgreSQL database
   - Nakama server with the Tic-Tac-Toe module loaded

3. Build the Go plugin:
   ```bash
   cd modules
   go build -buildmode=plugin -o backend.so
   ```

4. The Nakama server will be available at:
   - HTTP API: http://localhost:7350
   - gRPC API: localhost:7351

### Testing the API

You can test the RPC endpoints using curl or any HTTP client:

```bash
# Create a match
curl -X POST http://localhost:7350/v2/rpc/create_match \
  -H "Authorization: Bearer <session_token>"

# Get player stats
curl http://localhost:7350/v2/rpc/get_stats \
  -H "Authorization: Bearer <session_token>"
```

## Deploying to Google Cloud Platform (GCP)

### Option 1: Using Cloud Run

1. **Build and push Docker image:**
   ```bash
   # Build the plugin
   cd modules
   go build -buildmode=plugin -o backend.so

   # Create Dockerfile for Nakama with plugin
   # (See example below)

   # Build and push to GCR
   gcloud builds submit --tag gcr.io/YOUR_PROJECT/tic-tac-toe-nakama
   ```

2. **Deploy to Cloud Run:**
   ```bash
   gcloud run deploy tic-tac-toe-nakama \
     --image gcr.io/YOUR_PROJECT/tic-tac-toe-nakama \
     --platform managed \
     --port 7350 \
     --allow-unauthenticated \
     --set-env-vars "NAKAMA_DATABASE_ADDRESS=your-postgres-connection-string"
   ```

### Option 2: Using Google Kubernetes Engine (GKE)

1. **Create a GKE cluster:**
   ```bash
   gcloud container clusters create tic-tac-toe-cluster --num-nodes=2
   ```

2. **Deploy using Kubernetes manifests:**
   - Create PersistentVolumeClaim for PostgreSQL
   - Deploy PostgreSQL StatefulSet
   - Deploy Nakama Deployment with the plugin volume mounted

### Option 3: Using Compute Engine VM

1. **Create a VM instance:**
   ```bash
   gcloud compute instances create nakama-server \
     --image-family=ubuntu-2004-lts \
     --image-project=ubuntu-os-cloud \
     --machine-type=e2-medium
   ```

2. **Install Nakama and PostgreSQL on the VM:**
   ```bash
   # Install Docker
   sudo apt update
   sudo apt install docker.io

   # Run PostgreSQL container
   sudo docker run -d --name postgres -e POSTGRES_DB=nakama -e POSTGRES_PASSWORD=yourpassword -p 5432:5432 postgres:12

   # Build and run Nakama with plugin
   sudo docker run -d --name nakama \
     -p 7350:7350 -p 7351:7351 \
     -v /path/to/modules:/nakama/data/modules \
     heroiclabs/nakama:3.22.0 \
     /nakama/nakama --database.address postgres:yourpassword@localhost:5432/nakama
   ```

### Environment Variables

Set these environment variables for production:

- `NAKAMA_DATABASE_ADDRESS` - PostgreSQL connection string
- `NAKAMA_SESSION_TOKEN_EXPIRY_SEC` - Session token expiry
- `NAKAMA_LOG_LEVEL` - Logging level (INFO, DEBUG, etc.)

## Connecting from Godot

To connect your Godot game to the Nakama backend, use the Nakama Godot SDK.

### Installation

1. Download the Nakama Godot SDK from the [official repository](https://github.com/heroiclabs/nakama-godot).

2. Add the SDK files to your Godot project.

### Basic Connection

```gdscript
extends Node

var client = Nakama.create_client("defaultkey", "127.0.0.1", 7350, "http")
var session

func _ready():
    # Authenticate user
    session = yield(client.authenticate_email_async("email@example.com", "password"), "completed")
    
    # Create or join a match
    var match = yield(client.create_match_async(), "completed")
    
    # Connect to match
    var socket = Nakama.create_socket_from(client)
    yield(socket.connect_async(session), "completed")
    
    # Join the match
    yield(socket.join_match_async(match.match_id), "completed")
    
    # Send moves
    socket.send_match_state_async(match.match_id, 1, JSON.print({"row": 0, "col": 0}))

func _on_match_state(data):
    # Handle game state updates
    var state = JSON.parse(data.state).result
    # Update your game UI based on state
```

### Handling Match Events

```gdscript
func _on_match_state(data):
    var op_code = data.op_code
    var state = JSON.parse(data.state).result
    
    match op_code:
        1: # Move
            # Handle move
        2: # State update
            update_game_board(state.grid)
        3: # Game result
            show_game_result(state)

func update_game_board(grid):
    # Update your Tic-Tac-Toe board UI
    pass

func show_game_result(result):
    # Show win/lose/draw message
    pass
```

### Matchmaking

```gdscript
# Create matchmaking ticket
var ticket = yield(client.add_matchmaker_async("+properties.skill:>5 +properties.skill:<10", 2, 2), "completed")

# The matchmaker_matched signal will be emitted when a match is found
```

## Game Features

- Real-time Tic-Tac-Toe matches
- Player statistics (wins, losses, draws)
- Matchmaking support
- RPC endpoints for match creation and stats retrieval

## API Reference

### RPC Endpoints

- `create_match`: Creates a new Tic-Tac-Toe match
- `get_stats`: Retrieves player statistics

### Match Op Codes

- `1`: Player move (payload: `{"row": int, "col": int}`)
- `2`: Game state update
- `3`: Game result

## Contributing

1. Make changes to the Go code
2. Build the plugin: `go build -buildmode=plugin -o backend.so`
3. Test locally with Docker Compose
4. Commit and push changes

## License

This project is licensed under the MIT License.