# AES67 Go Daemon (aes67d) — Design Spec

**Date:** 2026-03-19
**Status:** Approved

## Overview

Complete rewrite of the AES67 Linux daemon in Go. Communicates directly with the RAVENNA kernel module via raw netlink sockets, serves the WebUI and REST API, handles SAP/mDNS discovery, SDP parsing, PTP monitoring, and audio stream management. Runs as root for kernel access, drops privileges internally for non-privileged components.

## Directory Structure

```
aes67d/
  main.go                    # Entry point, privilege management, signal handling
  go.mod
  go.sum
  cmd/
    aes67d/
      main.go                # CLI entry point with flags
  internal/
    netlink/
      netlink.go             # Raw netlink socket abstraction
      messages.go            # MT_ALSA_msg definitions, message IDs
      types.go               # TRTP_stream_info, TPTPConfig, TPTPStatus structs
    driver/
      driver.go              # DriverManager — send commands to kernel module
    session/
      session.go             # SessionManager — source/sink lifecycle
      source.go              # StreamSource type and operations
      sink.go                # StreamSink type and operations
      sdp.go                 # SDP parsing and generation
    ptp/
      ptp.go                 # PTP status monitoring, config
    discovery/
      sap.go                 # SAP announcement send/receive
      mdns.go                # mDNS/Avahi source discovery
    config/
      config.go              # JSON config file (compatible with existing daemon.conf)
    api/
      server.go              # HTTP server setup, middleware, static files
      routes.go              # Route registration
      config_handler.go      # GET/POST /api/config
      ptp_handler.go         # GET/POST /api/ptp/*
      source_handler.go      # CRUD /api/source/*
      sink_handler.go        # CRUD /api/sink/*
      browser_handler.go     # GET /api/browse/*
      matrix_handler.go      # GET/PUT /api/matrix/*
      streamer_handler.go    # Streamer endpoints (if enabled)
    privileges/
      privileges.go          # Drop root privileges, setuid/setgid
```

## Privilege Management

The daemon starts as root (required for raw netlink sockets to the kernel module). After opening the netlink sockets:

1. **Open netlink sockets** as root (protocols 31 and 29)
2. **Open any privileged ports** (if needed)
3. **Create aes67-daemon user/group** if not exists
4. **Drop to aes67-daemon user** via `syscall.Setgid()` + `syscall.Setuid()`
5. **Keep netlink file descriptors** — already opened, no re-auth needed

The HTTP server, config management, and all business logic run as unprivileged. Only the netlink sockets (opened before dropping) retain kernel access.

```go
func dropPrivileges(username string) error {
    u, err := user.Lookup(username)
    if err != nil { return err }
    uid, _ := strconv.Atoi(u.Uid)
    gid, _ := strconv.Atoi(u.Gid)
    if err := syscall.Setgid(gid); err != nil { return err }
    if err := syscall.Setuid(uid); err != nil { return err }
    return nil
}
```

## Netlink Communication

### Protocol

Two raw netlink sockets:
- **U2K (protocol 31)** — daemon → kernel commands (synchronous request/response)
- **K2U (protocol 29)** — kernel → daemon events (async, daemon listens continuously)

### Message Frame

```
[nlmsghdr (16 bytes)]       standard Linux netlink header
  nlmsg_len                 total message length
  nlmsg_type = NLMSG_DONE
  nlmsg_flags = 0
  nlmsg_pid = own PID
[MT_ALSA_msg (12 bytes)]    custom AES67 header
  id        (int32)         message type enum
  errCode   (int32)         0 = success, else error
  dataSize  (int32)         payload size
[Payload (0-1024 bytes)]    message-specific data
```

### Message Types

**Lifecycle:** Start(0), Stop(1), Reset(2), Hello(20), Bye(21)
**Audio:** SetSampleRate(5), GetSampleRate(6), SetTICFrameSize(9), SetMaxTICFrameSize(10), SetPlayoutDelay(30)
**Channels:** SetNumberOfInputs(11), SetNumberOfOutputs(12), SetInterfaceName(15)
**Streams:** AddRTPStream(16), RemoveRTPStream(17), UpdateStreamName(18), GetStreamStatus(29)
**PTP:** SetPTPConfig(31), GetPTPConfig(32), GetPTPStatus(33)
**Volume:** Set/GetMasterOutputVolume(23/25), Set/GetMasterOutputSwitch(24/26)

### Go Implementation

```go
type NetlinkConn struct {
    fd   int           // raw socket fd
    pid  uint32        // our PID for nlmsghdr
    mu   sync.Mutex    // serialize commands
}

func NewNetlinkConn(protocol int) (*NetlinkConn, error) {
    fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, protocol)
    // bind to PID, connect to kernel (pid=0)
    ...
}

func (c *NetlinkConn) Send(msgID int32, data []byte) ([]byte, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    // Build nlmsghdr + MT_ALSA_msg + payload
    // Send to kernel
    // Wait for response (1s timeout)
    // Parse response, check errCode
    // Return payload
}
```

### Key Data Structures (Go equivalents)

```go
// TRTP_stream_info — packed C struct, must match kernel layout exactly
type RTPStreamInfo struct {
    SizeOf            uint32
    Is8021Q           int8
    VLANId            uint16
    IfPortId          uint32
    Name              [64]byte
    PlayOutDelay      uint32
    FrameSize         uint32
    MaxSamplesPerPkt  uint32
    DestMAC           [6]byte
    DSCP              uint8
    RTCPSrcIP         uint32
    SrcIP             uint32
    DestIP            uint32
    TTL               uint8
    SrcPort           uint16
    DestPort          uint16
    RTCPSrcPort       uint16
    RTCPDestPort      uint16
    PayloadType       uint8
    SSRC              uint32
    SSRCInitialized   int8
    RTPTimestampOff   uint32
    SamplingRate      uint32
    Codec             [10]byte
    WordLength        uint8
    NbOfChannels      uint8
    IsSource          int8
    Id                uint32
    Routing           [64]uint32
}

type PTPConfig struct {
    Domain uint8
    DSCP   uint8
}

type PTPStatus struct {
    LockStatus int32   // 0=unlocked, 1=locking, 2=locked
    GMID       uint64
    Jitter     int32
}

type StreamStatus struct {
    Flags   uint32  // bit fields
    MinTime int32
}
```

**Critical:** All structs must use `encoding/binary` with `LittleEndian` (ARM64) byte order and match the C struct layout exactly including padding. Use `#pragma pack(push, 1)` equivalent via struct tags or manual serialization.

## Driver Manager

Wraps netlink communication into a high-level Go API:

```go
type Driver struct {
    u2k *NetlinkConn  // commands
    k2u *NetlinkConn  // events
}

func (d *Driver) Hello() error
func (d *Driver) Start() error
func (d *Driver) Stop() error
func (d *Driver) Reset() error
func (d *Driver) SetSampleRate(rate uint32) error
func (d *Driver) GetSampleRate() (uint32, error)
func (d *Driver) SetTICFrameSize(size uint64) error
func (d *Driver) SetMaxTICFrameSize(size uint64) error
func (d *Driver) SetPlayoutDelay(delay int32) error
func (d *Driver) SetInterfaceName(name string) error
func (d *Driver) SetNumberOfInputs(n uint32) error
func (d *Driver) SetNumberOfOutputs(n uint32) error
func (d *Driver) AddRTPStream(info *RTPStreamInfo) (uint64, error)  // returns handle
func (d *Driver) RemoveRTPStream(handle uint64) error
func (d *Driver) GetStreamStatus(handle uint64) (*StreamStatus, error)
func (d *Driver) SetPTPConfig(cfg *PTPConfig) error
func (d *Driver) GetPTPConfig() (*PTPConfig, error)
func (d *Driver) GetPTPStatus() (*PTPStatus, error)
```

The K2U event listener runs in a goroutine, dispatching events to registered handlers.

## Session Manager

High-level source/sink management:

```go
type SessionManager struct {
    driver   *Driver
    config   *Config
    sources  map[uint8]*StreamInfo  // id → stream info + handle
    sinks    map[uint8]*StreamInfo
    mu       sync.RWMutex
}

func (sm *SessionManager) AddSource(src StreamSource) error
func (sm *SessionManager) RemoveSource(id uint8) error
func (sm *SessionManager) GetSources() []StreamSource
func (sm *SessionManager) GetSource(id uint8) (StreamSource, error)

func (sm *SessionManager) AddSink(sink StreamSink) error
func (sm *SessionManager) RemoveSink(id uint8) error
func (sm *SessionManager) GetSinks() []StreamSink
func (sm *SessionManager) GetSink(id uint8) (StreamSink, error)
func (sm *SessionManager) GetSinkStatus(id uint8) (*SinkStatus, error)
```

StreamSource/StreamSink types mirror the JSON format of the existing daemon.conf for compatibility.

## Config

Compatible with the existing `daemon.conf` JSON format. Same field names, same semantics. The Go daemon reads the same config file the C++ daemon uses.

```go
type Config struct {
    HTTPPort        int    `json:"http_port"`
    RTSPPort        int    `json:"rtsp_port"`
    HTTPBaseDir     string `json:"http_base_dir"`
    LogSeverity     int    `json:"log_severity"`
    PlayoutDelay    int    `json:"playout_delay"`
    TICFrameSize    int    `json:"tic_frame_size_at_1fs"`
    MaxTICFrameSize int    `json:"max_tic_frame_size"`
    SampleRate      int    `json:"sample_rate"`
    RTPMcastBase    string `json:"rtp_mcast_base"`
    RTPPort         int    `json:"rtp_port"`
    PTPDomain       int    `json:"ptp_domain"`
    PTPDSCP         int    `json:"ptp_dscp"`
    SAPMcastAddr    string `json:"sap_mcast_addr"`
    SAPInterval     int    `json:"sap_interval"`
    SyslogProto     string `json:"syslog_proto"`
    SyslogServer    string `json:"syslog_server"`
    StatusFile      string `json:"status_file"`
    InterfaceName   string `json:"interface_name"`
    MDNSEnabled     bool   `json:"mdns_enabled"`
    CustomNodeID    string `json:"custom_node_id"`
    PTPStatusScript string `json:"ptp_status_script"`
    StreamerChannels    int  `json:"streamer_channels"`
    StreamerFilesNum    int  `json:"streamer_files_num"`
    StreamerFileDuration int `json:"streamer_file_duration"`
    StreamerBufferFiles int  `json:"streamer_player_buffer_files_num"`
    StreamerEnabled     bool `json:"streamer_enabled"`
    AutoSinksUpdate     bool `json:"auto_sinks_update"`
    // Computed at runtime
    NodeID    string `json:"node_id"`
    IPAddr    string `json:"ip_addr"`
    MACAddr   string `json:"mac_addr"`
}
```

## HTTP API

Uses Go standard library `net/http` with a lightweight router (chi or just `http.ServeMux` from Go 1.22+).

All existing API endpoints preserved with identical request/response formats:

- `GET/POST /api/config`
- `GET /api/version`
- `GET/POST /api/ptp/config`, `GET /api/ptp/status`
- `GET /api/sources`, `PUT/DELETE /api/source/{id}`
- `GET /api/sinks`, `PUT/DELETE /api/sink/{id}`, `GET /api/sink/status/{id}`
- `GET /api/browse/sources/{all|sap|mdns}`
- `GET/PUT /api/matrix`, `PUT /api/matrix/route`
- `GET /api/source/sdp/{id}`
- Static file serving for WebUI from `http_base_dir`

SPA catch-all: any non-API, non-static route serves `index.html`.

CORS headers: `Access-Control-Allow-Origin: *`, standard methods.

## SAP Discovery

SAP (Session Announcement Protocol) for AES67 source discovery:
- Listen on multicast `239.255.255.255:9875` for announcements
- Parse SDP from SAP payloads
- Announce local sources periodically
- Go `net` package for multicast UDP

## mDNS Discovery

Use `github.com/hashicorp/mdns` or similar for mDNS/DNS-SD:
- Browse for `_ravenna._udp` and `_rtsp._tcp` services
- Register local sources as mDNS services when `mdns_enabled`

## SDP Parsing

Minimal SDP parser for AES67 streams:
- Parse `v=`, `o=`, `s=`, `c=`, `m=`, `a=` lines
- Extract: multicast address, port, codec, sample rate, channel count, PTP clock reference
- Generate SDP for local sources

## Systemd Integration

- `sd_notify` for Type=notify service (use `github.com/coreos/go-systemd/v22/daemon`)
- Watchdog support
- Journal logging via stderr (systemd captures)

## Build

```bash
cd aes67d
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o aes67d ./cmd/aes67d
```

Cross-compiles cleanly for ARM64 (RPi5) without any C dependencies.

## Migration

- Drop-in replacement for the C++ daemon binary
- Same config file format
- Same systemd service (just change `ExecStart` path)
- Same WebUI (served from `http_base_dir`)
- Same API (identical endpoints and JSON formats)
