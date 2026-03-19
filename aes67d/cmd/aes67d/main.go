package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/api"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/config"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/discovery"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/driver"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/privileges"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/ptp"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/session"
)

var version = "dev"

func main() {
	// CLI flags
	configPath := flag.String("c", "/etc/daemon.conf", "config file path")
	httpAddr := flag.String("a", "0.0.0.0", "HTTP bind address")
	httpPort := flag.Int("p", 0, "HTTP port (overrides config, default 8900)")
	ifaceName := flag.String("i", "", "network interface (overrides config)")
	debug := flag.Bool("d", false, "enable debug logging")
	showVersion := flag.Bool("v", false, "print version and exit")
	noPrivDrop := flag.Bool("no-priv-drop", false, "don't drop root privileges")
	privUser := flag.String("u", "aes67-daemon", "user to drop privileges to")
	flag.Parse()

	if *showVersion {
		fmt.Printf("aes67d %s\n", version)
		os.Exit(0)
	}

	log.Printf("aes67d %s starting...", version)

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: could not load config %s: %v (using defaults)", *configPath, err)
		cfg = config.DefaultConfig()
	}

	// CLI overrides
	if *httpPort > 0 {
		cfg.HTTPPort = *httpPort
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8900
	}
	if *ifaceName != "" {
		cfg.InterfaceName = *ifaceName
	}

	// Populate runtime info (IP, MAC, NodeID)
	if err := cfg.PopulateRuntime(); err != nil {
		log.Printf("Warning: could not populate runtime info: %v", err)
	}

	if *debug {
		log.Printf("Config loaded: %+v", cfg)
	}

	// Open netlink sockets (requires root)
	drv, err := driver.New()
	if err != nil {
		log.Fatalf("Failed to open netlink sockets: %v (are you root?)", err)
	}
	defer drv.Close()

	// Drop privileges
	if privileges.IsRoot() && !*noPrivDrop {
		log.Printf("Dropping privileges to user %q...", *privUser)
		if err := privileges.Drop(*privUser); err != nil {
			log.Printf("Warning: could not drop privileges: %v (continuing as root)", err)
		} else {
			log.Printf("Privileges dropped successfully")
		}
	}

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize driver
	log.Println("Initializing RAVENNA driver...")
	if err := drv.Hello(); err != nil {
		log.Fatalf("Driver hello failed: %v", err)
	}
	if err := drv.Start(); err != nil {
		log.Fatalf("Driver start failed: %v", err)
	}

	// Configure driver from config
	drv.SetSampleRate(uint32(cfg.SampleRate))
	drv.SetTICFrameSize(uint64(cfg.TICFrameSize))
	drv.SetMaxTICFrameSize(uint64(cfg.MaxTICFrameSize))
	drv.SetPlayoutDelay(int32(cfg.PlayoutDelay))
	drv.SetInterfaceName(cfg.InterfaceName)

	// PTP monitor
	ptpMon := ptp.NewMonitor(drv, 500*time.Millisecond)
	ptpMon.InitConfig(cfg.PTPDomain, cfg.PTPDSCP)
	ptpMon.OnChange(func(status string) {
		log.Printf("PTP status changed: %s", status)
	})
	go ptpMon.Start(ctx)

	// Session manager
	sessMgr := session.NewManager(drv, cfg)
	if err := sessMgr.Init(); err != nil {
		log.Printf("Warning: session init: %v", err)
	}

	// Discovery
	disc := discovery.NewManager(cfg.InterfaceName, cfg.SAPInterval, cfg.MDNSEnabled)
	go disc.Start(ctx)

	// K2U event listener
	go drv.ListenEvents(ctx)

	// HTTP server
	addr := fmt.Sprintf("%s:%d", *httpAddr, cfg.HTTPPort)
	srv := api.NewServer(cfg, sessMgr, ptpMon, disc, version)

	// Systemd notify (optional, best-effort)
	notifySystemd("READY=1")
	log.Printf("HTTP server listening on %s", addr)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			log.Printf("HTTP server error: %v", err)
			cancel()
		}
	}()

	<-sigCh
	log.Println("Shutting down...")
	notifySystemd("STOPPING=1")
	cancel()
	drv.Bye()
	log.Println("Goodbye.")
}

func notifySystemd(state string) {
	// Best-effort sd_notify via NOTIFY_SOCKET
	addr := os.Getenv("NOTIFY_SOCKET")
	if addr == "" {
		return
	}
	conn, err := net.Dial("unixgram", addr)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write([]byte(state))
}
