# HTMX Space Invaders

A Space Invaders game built with HTMX and server-side rendering, demonstrating that complex real-time games can be built without client-side JavaScript game logic.

## Features

- **Pure HTMX + Server-Side Rendering**: All game logic runs on the server
- **WebSocket-based real-time updates**: Using HTMX WebSocket extension for live gameplay
- **60 FPS game loop**: Smooth gameplay tuned for desktop and mobile devices
- **Mobile-first design**: Touch controls with modern neumorphic button design
- **Frame acknowledgment system**: Tracks client FPS and network latency
- **Keyboard and touch support**: Works on desktop and mobile devices

## Technology Stack

- **Frontend**: HTMX 2.0.0 with WebSocket extension
- **Backend**: Go with Gorilla WebSocket
- **Graphics**: Custom SVG sprites
- **Styling**: CSS with neumorphic design

## Running the Game

1. Install Go (1.19 or higher)
2. Clone the repository
3. Run the server:
   ```bash
   go run .
   ```
4. Open http://localhost:8080 in your browser

## Controls

- **Desktop**: Arrow keys to move, Space to shoot
- **Mobile**: On-screen touch controls (automatically displayed)

## Architecture

The game demonstrates an unconventional but interesting architecture:

- **Server maintains all game state**: Player position, alien position, bullets, collisions
- **Client sends only input events**: Key presses/releases via WebSocket
- **Server streams targeted HTML fragments**: Position updates ship via CSS transforms while counters use out-of-band swaps
- **HTMX handles DOM updates**: Minimal fragments keep the client workload light

### Performance Metrics

The game displays three metrics in the top-right corner:
- **S**: Server FPS (how fast the server generates frames)
- **C**: Client FPS (how fast the client acknowledges frames)
- **L**: Latency in milliseconds (round-trip time)

## Project Structure

```
htmx-game/
├── main.go           # HTTP server and routing
├── game.go           # Game logic and state
├── websocket.go      # WebSocket connection handling
├── index.html        # Main HTML with HTMX setup
├── static/
│   ├── styles.css    # Game and control styling
│   ├── spaceship.svg # Player sprite
│   ├── alien.svg     # Enemy sprite
│   ├── bullet.svg    # Projectile sprite
│   ├── explosion-*.svg # Explosion animations
│   └── *.svg         # Control icons
└── go.mod           # Go dependencies
```

## Design Highlights

- **Neumorphic controls**: Modern, tactile button design with depth
- **Custom SVG icons**: Perfectly centered arrow and crosshair icons
- **Responsive layout**: Fixed 360x480px game canvas optimized for mobile
- **Visual feedback**: Press animations and distinct fire button color

## Technical Notes

- The game uses a frame-pacing approach where new frames are generated as fast as the network allows
- Delta time calculations ensure consistent physics regardless of frame rate
- The WebSocket connection uses JSON messages for input and HTML for output
- All game objects are absolutely positioned within a container div

## License

MIT

## Acknowledgments

Built to explore the limits of HTMX and server-side rendering for real-time applications.