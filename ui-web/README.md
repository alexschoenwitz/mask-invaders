# Mask Invaders - Web UI

A browser-based UI for the Mask Invaders game using HTML5 Canvas.

## Features

- ✅ Real-time game visualization
- ✅ Click-to-select castle interaction
- ✅ Attack and Produce actions
- ✅ Troop movement visualization
- ✅ Player color coding
- ✅ No build tools required - pure HTML/JS

## How to Run

1. **Start the game server:**
   ```bash
   cd server
   go run .
   ```

2. **Serve the web UI:**
   
   You can use any static file server. Here are a few options:
   
   **Option A - Python:**
   ```bash
   cd ui-web
   python3 -m http.server 8000
   ```
   
   **Option B - Node.js:**
   ```bash
   cd ui-web
   npx http-server -p 8000
   ```
   
   **Option C - Go:**
   ```bash
   cd ui-web
   go run -m http.server -p 8000
   ```

3. **Open in browser:**
   ```
   http://localhost:8000
   ```

4. **Join the game:**
   - Enter your player name
   - Keep the server URL as `http://localhost:8080`
   - Click "Join Game"
   - Press 'S' to start the game

## Controls

- **Click castle**: Select your castle
- **Click ATTACK**: Choose attack mode, then click target
- **Click PRODUCE**: Choose troop type (A/B/C)
- **Press 'S'**: Start the game
- **Press 'ESC'**: Cancel current action

## Multiple Players

Open multiple browser windows/tabs with different player names to play against each other!

## CORS Note

If you get CORS errors, the server needs to allow cross-origin requests. You can run the UI from a file:// URL or add CORS headers to the server.
