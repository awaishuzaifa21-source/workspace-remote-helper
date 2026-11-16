// Workspace Remote Helper.
//
// A small, visible program the customer runs so a signed-in technician can move the mouse
// and type during one remote support session. It:
//   - listens only on this computer (127.0.0.1), and only answers the Workspace website;
//   - asks the person at the computer to approve control in a system dialog, every time;
//   - stops when the web page says stop, when this window is closed, or after 20 seconds
//     without contact; and never installs itself, starts on boot or runs hidden.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const version = "1.1.0"

type event struct {
	T    string  `json:"t"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	B    int     `json:"b"`
	DY   float64 `json:"dy"`
	Code string  `json:"code"`
	Down bool    `json:"down"`
}

var (
	mu       sync.Mutex
	token    string
	lastSeen time.Time
	held     = map[string]bool{} // keys and buttons currently down, released on stop
	allowed  = map[string]bool{"https://huzaifaawais.me": true, "https://www.huzaifaawais.me": true}
	port     = 47821
)

func revoke(why string) {
	mu.Lock()
	active := token != ""
	token = ""
	keys := held
	held = map[string]bool{}
	mu.Unlock()
	for k := range keys {
		if strings.HasPrefix(k, "btn") {
			inject(event{T: "up", B: int(k[3] - '0'), X: -1})
		} else {
			inject(event{T: "key", Code: k, Down: false})
		}
	}
	if active {
		fmt.Println(time.Now().Format("15:04:05"), "Remote control stopped:", why)
	}
}

// cors answers only the Workspace website, and only when addressed as localhost.
func cors(w http.ResponseWriter, r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.Host)
	origin := r.Header.Get("Origin")
	if (host != "127.0.0.1" && host != "localhost") || !allowed[origin] {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Vary", "Origin")
	h.Set("Access-Control-Allow-Headers", "Content-Type, X-Token")
	h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	h.Set("Access-Control-Allow-Private-Network", "true")
	h.Set("Access-Control-Max-Age", "600")
	h.Set("Cache-Control", "no-store")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	return true
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func authed(r *http.Request) bool {
	mu.Lock()
	defer mu.Unlock()
	sent := r.Header.Get("X-Token")
	if token == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
		return false
	}
	lastSeen = time.Now()
	return true
}

func clean(s string) string {
	out := make([]rune, 0, 60)
	for _, c := range s {
		if c >= 32 && c != 127 && len(out) < 60 {
			out = append(out, c)
		}
	}
	return strings.TrimSpace(string(out))
}

func main() {
	extra := flag.String("origin", "", "extra allowed website origin (for testing)")
	flag.IntVar(&port, "port", port, "local port")
	flag.Parse()
	if *extra != "" {
		allowed[*extra] = true
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		if !cors(w, r) {
			return
		}
		mu.Lock()
		active := token != ""
		mu.Unlock()
		reply(w, 200, map[string]any{"app": "workspace-helper", "version": version, "os": runtime.GOOS, "control": canInject(), "note": platformNote(), "active": active})
	})
	mux.HandleFunc("/grant", func(w http.ResponseWriter, r *http.Request) {
		if !cors(w, r) {
			return
		}
		if r.Method != http.MethodPost || !canInject() {
			reply(w, 405, map[string]string{"error": "Control is not available on this computer."})
			return
		}
		var in struct{ Tech, Title string }
		json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in)
		tech, title := clean(in.Tech), clean(in.Title)
		if tech == "" {
			tech = "The technician"
		}
		revoke("a new request arrived")
		if !confirm(tech, title) {
			fmt.Println(time.Now().Format("15:04:05"), "Remote control request declined.")
			reply(w, 403, map[string]string{"error": "You declined remote control."})
			return
		}
		b := make([]byte, 24)
		rand.Read(b)
		mu.Lock()
		token = hex.EncodeToString(b)
		lastSeen = time.Now()
		t := token
		mu.Unlock()
		fmt.Println(time.Now().Format("15:04:05"), "Remote control ALLOWED for", tech+". Close this window to stop.")
		reply(w, 200, map[string]string{"token": t})
	})
	mux.HandleFunc("/input", func(w http.ResponseWriter, r *http.Request) {
		if !cors(w, r) {
			return
		}
		if !authed(r) {
			reply(w, 401, map[string]string{"error": "not allowed"})
			return
		}
		var evs []event
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&evs); err != nil || len(evs) > 300 {
			reply(w, 400, map[string]string{"error": "bad input"})
			return
		}
		for _, e := range evs {
			switch e.T {
			case "move", "wheel":
			case "down", "up":
				if e.B < 0 || e.B > 2 {
					continue
				}
				mu.Lock()
				if e.T == "down" {
					held[fmt.Sprintf("btn%d", e.B)] = true
				} else {
					delete(held, fmt.Sprintf("btn%d", e.B))
				}
				mu.Unlock()
			case "key":
				if _, ok := keyCodes[e.Code]; !ok {
					continue
				}
				mu.Lock()
				if e.Down {
					held[e.Code] = true
				} else {
					delete(held, e.Code)
				}
				mu.Unlock()
			default:
				continue
			}
			inject(e)
		}
		reply(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if !cors(w, r) {
			return
		}
		reply(w, 200, map[string]bool{"ok": authed(r)})
	})
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if !cors(w, r) {
			return
		}
		revoke("stopped from the support page")
		reply(w, 200, map[string]bool{"ok": true})
	})

	go func() {
		for range time.Tick(2 * time.Second) {
			mu.Lock()
			stale := token != "" && time.Since(lastSeen) > 20*time.Second
			mu.Unlock()
			if stale {
				revoke("no contact from the support page for 20 seconds")
			}
		}
	}()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Println("The helper is already running, or the port is busy:", err)
		time.Sleep(6 * time.Second)
		os.Exit(1)
	}
	fmt.Println("Workspace Remote Helper", version)
	fmt.Println("------------------------------------------------------------")
	fmt.Println("This lets a technician you trust move your mouse and type,")
	fmt.Println("only after you click Yes in the dialog that will appear.")
	fmt.Println()
	fmt.Println("  * Keep this window open during the support session.")
	fmt.Println("  * CLOSE THIS WINDOW at any time to stop remote control.")
	fmt.Println("  * Nothing is installed. It does not start again by itself.")
	fmt.Println("------------------------------------------------------------")
	fmt.Println("Waiting for the support page...")
	http.Serve(ln, mux)
}
