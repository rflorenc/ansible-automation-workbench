package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	workbench "github.com/rflorenc/ansible-automation-workbench"
	"github.com/rflorenc/ansible-automation-workbench/internal/api"
	"github.com/rflorenc/ansible-automation-workbench/internal/config"
	"github.com/rflorenc/ansible-automation-workbench/internal/models"
	"github.com/rflorenc/ansible-automation-workbench/internal/platform"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("workbench %s (commit: %s, built: %s)\n", version, commit, date)
			os.Exit(0)
		}
	}

	cfg := config.Parse()

	server := &api.Server{
		Connections: models.NewConnectionStore(),
		Jobs:        models.NewJobStore(),
		Previews:    api.NewPreviewStore(),
	}

	// Load pre-configured connections from config file
	for _, cc := range cfg.Connections {
		conn := &models.Connection{
			Name:     cc.Name,
			Type:     cc.Type,
			Role:     cc.Role,
			Scheme:   cc.Scheme,
			Host:     cc.Host,
			Port:     cc.Port,
			Username: cc.Username,
			Password: cc.Password,
			Insecure: cc.Insecure,
		}
		if conn.Role == "" {
			if conn.Type == "awx" {
				conn.Role = "source"
			} else {
				conn.Role = "destination"
			}
		}
		if conn.Scheme == "" {
			if conn.Type == "aap" {
				conn.Scheme = "https"
			} else {
				conn.Scheme = "http"
			}
		}
		if conn.Port == 0 {
			if conn.Scheme == "https" {
				conn.Port = 443
			} else {
				conn.Port = 80
			}
		}
		server.Connections.Create(conn)
		fmt.Printf("Loaded connection: %s (%s://%s:%d)\n", conn.Name, conn.Scheme, conn.Host, conn.Port)

		// Verify connectivity and auth early
		p := platform.NewPlatform(conn)
		client := platform.NewClient(conn)
		pingStatus, pingError := "ok", ""
		if err := p.Ping(); err != nil {
			pingStatus = "error"
			pingError = err.Error()
			fmt.Printf("  PING FAILED: %s: %v\n", conn.Name, err)
		} else {
			fmt.Printf("  PING OK: %s: reachable\n", conn.Name)
		}

		authStatus, authError := "unknown", ""
		if pingStatus == "ok" {
			if conn.Username == "" || conn.Password == "" {
				authStatus = "error"
				authError = "no credentials configured"
				fmt.Printf("  AUTH FAILED: %s: %s\n", conn.Name, authError)
			} else if err := p.CheckAuth(); err != nil {
				authStatus = "error"
				authError = err.Error()
				fmt.Printf("  AUTH FAILED: %s: %v\n", conn.Name, err)
			} else {
				authStatus = "ok"
				fmt.Printf("  AUTH OK: %s: authenticated successfully\n", conn.Name)

				// Discovery: detect version and API prefix (only after auth succeeds)
				pingResp, err := client.PingWithVersion(platform.PingPath(conn.Type))
				if err == nil && pingResp.Version != "" {
					conn.Version = pingResp.Version
					server.Connections.SetVersion(conn.ID, pingResp.Version, "")
					fmt.Printf("  VERSION: %s: %s\n", conn.Name, pingResp.Version)
				}
				platform.DiscoverAndStore(client, conn, server.Connections)
			}
		}
		server.Connections.SetHealth(conn.ID, pingStatus, pingError, authStatus, authError)
	}

	var webFS fs.FS
	if cfg.Dev {
		// In dev mode, proxy to Vite dev server
		webFS = nil
	} else {
		// Use embedded filesystem
		var err error
		webFS, err = fs.Sub(workbench.WebFS, "web/dist")
		if err != nil {
			log.Fatal("Failed to get embedded web FS: ", err)
		}
	}

	var handler http.Handler
	if cfg.Dev {
		// In dev mode, create router with a proxy to Vite
		handler = devRouter(server)
	} else {
		handler = api.NewRouter(server, webFS)
	}

	fmt.Printf("Ansible Automation Workbench %s starting on %s\n", version, cfg.Listen)
	if cfg.Dev {
		fmt.Println("Dev mode: proxying frontend to http://localhost:5173")
	}
	fmt.Printf("Open http://localhost%s in your browser\n", cfg.Listen)

	if err := http.ListenAndServe(cfg.Listen, handler); err != nil {
		log.Fatal(err)
	}
}

// devRouter creates a handler that serves API routes directly and proxies
// everything else to the Vite dev server.
func devRouter(server *api.Server) http.Handler {
	// Create API router with a dummy filesystem (won't be used for static files)
	apiRouter := api.NewRouter(server, emptyFS{})

	// Proxy to Vite dev server
	viteURL, _ := url.Parse("http://localhost:5173")
	proxy := httputil.NewSingleHostReverseProxy(viteURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route /api/* and /ws/* to our Go server
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			apiRouter.ServeHTTP(w, r)
			return
		}
		if len(r.URL.Path) >= 3 && r.URL.Path[:3] == "/ws" {
			apiRouter.ServeHTTP(w, r)
			return
		}
		// Everything else goes to Vite
		proxy.ServeHTTP(w, r)
	})
}

// emptyFS is a minimal fs.FS that always returns not-found.
type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	return nil, os.ErrNotExist
}
