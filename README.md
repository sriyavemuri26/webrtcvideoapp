# WebRTC Video App
 
> A real-time, multi-party peer-to-peer video conferencing application built with a lightweight **Go** signaling server, native JavaScript **WebRTC API**, and concurrent **WebSockets**.
 
---
 
## What the Project Does
 
`webrtcvideoapp` is a decentralized, mesh-networked video communication platform that allows multiple clients to connect, share audio/video streams, and automatically manage dynamic peer lifecycles in real time.
 
* **Multi-User Video Conferencing**: Connects multiple users concurrently by instantiating isolated peer connections between every participant pair.
* **Asynchronous Signaling Pipeline**: Handles complex SDP exchange (offers/answers) and ICE candidate delivery through a centralized Go WebSocket server.
* **Dynamic Lifecycle Management**: Automatically handles media stream captures, media device constraints, client joins, and remote peer teardowns upon disconnect.
---
 
## How It Works
 
1. **Client Initialization & Media Capture**:
   Upon connecting to `ws://localhost:8080/ws`, the browser prompts the user for camera and microphone access (`getUserMedia`). Capture constraints default to 320x240 @ 15fps for optimized stream transport.
2. **Session Registration & ID Assignment**:
   The Go backend upgrades the HTTP connection to a persistent WebSocket, logs the client's network address as a unique `clientID`, and sends it back to the frontend.
3. **Automated Peer Mesh Creation**:
   When a new client joins an active session:
   * The Go server emits `create_pc` and `create_offer` control signals to both the new participant and existing peers.
   * Clients generate local `RTCPeerConnection` instances configured with Google's public STUN server (`stun:stun.l.google.com:19302`).
4. **SDP & ICE Candidate Handshake**:
   * Participants exchange Session Description Protocol (SDP) payloads (`offer` and `answer`) via the Go server.
   * ICE candidates are gathered asynchronously, queued, and dispatched once gathering completes (`iceGatheringState === 'complete'`).
5. **Stream Rendering & Teardown**:
   * Media tracks received on remote peer connections dynamically create `<video>` elements inside `#remoteVideos`.
   * Disconnections trigger a `client_disconnect` broadcast, prompting remaining clients to close specific peer connections and clean up the DOM.
---
 
## Architecture
 
```mermaid
sequenceDiagram
    autonumber
    actor Client A (Peer 1)
    participant Go Backend (Signaling Server)
    actor Client B (Peer 2)
 
    Client A->>Go Backend: WS Connect (/ws)
    Go Backend-->>Client A: Assign Client ID
 
    Client B->>Go Backend: WS Connect (/ws)
    Go Backend-->>Client B: Assign Client ID
 
    Note over Go Backend: Peer Mesh Triggered (numClients > 1)
    Go Backend-->>Client A: Signal: create_pc (Target: B)
    Go Backend-->>Client B: Signal: create_pc (Target: A)
    Go Backend-->>Client B: Signal: create_offer (To: A)
 
    Client B->>Go Backend: Send Offer SDP (To: A)
    Go Backend->>Client A: Relay Offer SDP
    Client A->>Go Backend: Send Answer SDP (To: B)
    Go Backend->>Client B: Relay Answer SDP
 
    Client B->>Go Backend: Send ICE Candidates (To: A)
    Go Backend->>Client A: Relay ICE Candidates
    Client A->>Go Backend: Send ICE Candidates (To: B)
    Go Backend->>Client B: Relay ICE Candidates
 
    Note over Client A,Client B: Direct P2P Media Stream Established (WebRTC)
```
 
---
 
## Tech Stack
 
* **Backend**: Go (`net/http`, `sync.Mutex` for thread-safe state management)
* **WebSocket Library**: Gorilla WebSocket (`github.com/gorilla/websocket`)
* **Frontend**: Vanilla JavaScript (ES6+ Async/Await, MediaDevices API, WebRTC API)
* **NAT Traversal**: STUN (`stun:stun.l.google.com:19302`)
* **Protocols**: WebSockets (Signaling), SRTP/ICE/STUN (P2P Streaming)
---
 
## Prerequisites
 
* **Go**: Version 1.22.4 or higher installed on your system.
---
 
## What I Learned
 
* **Thread-Safe WebSocket Management**: Implemented Go sync primitives (`sync.Mutex`) to safely manage concurrent writes and avoid race conditions when broadcasting signaling structs (`Signal`) across connected client maps.
* **Asynchronous WebRTC State Synchronization**: Built a robust front-end signal queue (`signalQueue`) to cache incoming offer/answer signals and ICE candidates until local media initialization (`getUserMedia`) finishes.
* **Batch ICE Candidate Gathering**: Handled candidate exchange using promises (`waitForIceCandidates`) to bundle and transfer candidates cleanly once gathering reaches a complete state.
* **Dynamic DOM & Track Cleanup**: Managed resource teardown cycles by properly unbinding `MediaStream` tracks and destroying dynamic video element IDs on client disconnects.

---
 
## Installation & Setup
 
**1. Clone the Repository:**
```bash
git clone https://github.com/sriyavemuri26/webrtcvideoapp.git
cd webrtcvideoapp
```
 
**2. Sync Dependencies:**
```bash
go mod tidy
```
 
**3. Build the Application:**
```bash
go build
```
 
**4. Run the Executable:**
 
Windows:
```powershell
./video-app.exe
```
 
macOS / Linux:
```bash
./webrtcvideoapp
```
 
**5. Access the Application:**
Open [http://localhost:8080](http://localhost:8080) in your web browser. Open a second tab or incognito window at [http://localhost:8080](http://localhost:8080) to test the peer-to-peer video connection!