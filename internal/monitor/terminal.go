package monitor

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// terminalEnabled reports whether the embedded web terminal is on. The
// terminal spawns an interactive shell as the service user, so it is
// strictly opt-in via MONITOR_TERMINAL=1 and inherits whatever protection
// fronts the dashboard (LAN-only in the current deployment).
func terminalEnabled() bool {
	return os.Getenv("MONITOR_TERMINAL") == "1"
}

var termUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Same exposure as the page itself; the page is expected to sit behind
	// the network boundary that governs access to the whole dashboard.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// TerminalHandler bridges one WebSocket connection to an interactive PTY
// shell. Binary frames carry terminal I/O; text frames carry JSON control
// messages (resize). Disconnecting kills the shell — no orphan sessions.
func (s *Server) TerminalHandler(w http.ResponseWriter, r *http.Request) {
	if !s.terminal {
		http.Error(w, "terminal disabled", http.StatusNotFound)
		return
	}
	ws, err := termUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	shell := "bash"
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		// Tell the client why instead of closing silently (e.g. the
		// unsupported-platform stub on Windows dev machines)
		ws.WriteMessage(websocket.TextMessage, []byte("terminal unavailable: "+err.Error()))
		return
	}
	defer ptmx.Close()

	// PTY output -> WebSocket
	go func() {
		defer ws.Close()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if ws.WriteMessage(websocket.BinaryMessage, buf[:n]) != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// WebSocket -> PTY input and control messages
	for {
		msgType, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		switch msgType {
		case websocket.BinaryMessage:
			ptmx.Write(data)
		case websocket.TextMessage:
			var ctl struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if json.Unmarshal(data, &ctl) == nil && ctl.Type == "resize" {
				_ = pty.Setsize(ptmx, &pty.Winsize{Rows: ctl.Rows, Cols: ctl.Cols})
			}
		}
	}

	cmd.Process.Kill()
	cmd.Wait()
}
