package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/weeaa/greenlight/pkg/page"
)

const DefaultMacOSChromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

type Browser struct {
	execPath     string
	wsEndpoint   string
	conn         *websocket.Conn
	cmd          *exec.Cmd
	context      context.Context
	cancel       context.CancelFunc
	userDataDir  string
	messageID    int
	messageMutex sync.Mutex
	PID          int
	isHeadless   bool
}

func GreenLight(ctx context.Context, execPath string, isHeadless bool, startURL string) (*Browser, error) {
	ctx, cancel := context.WithCancel(ctx)
	userDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("greenlight_%s", uuid.New().String()))

	browser := &Browser{
		execPath:    execPath,
		context:     ctx,
		cancel:      cancel,
		userDataDir: userDataDir,
		isHeadless:  isHeadless,
	}

	if err := browser.launch(startURL); err != nil {
		return nil, err
	}

	return browser, nil
}

func (b *Browser) launch(startURL string) error {
	args := []string{
		"--remote-debugging-port=" + os.Getenv("DEBUG_PORT"),
		"--no-first-run",
		"--user-data-dir=" + b.userDataDir,
		"--remote-allow-origins=*",
		startURL,
	}

	if b.isHeadless {
		args = append(args, "--headless=new")
	}

	b.cmd = exec.CommandContext(b.context, b.execPath, args...)
	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start browser: %v", err)
	}

	b.PID = b.cmd.Process.Pid

	time.Sleep(time.Second)

	return b.attachToPage()
}

func (b *Browser) attachToPage() error {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/json", os.Getenv("DEBUG_PORT")))
	if err != nil {
		return fmt.Errorf("failed to fetch active pages: %w", err)
	}
	defer resp.Body.Close()

	var pages []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pages); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	for _, page := range pages {
		if page["type"] == "page" && page["url"] != "" {
			if wsURL, ok := page["webSocketDebuggerUrl"].(string); ok {
				if b.conn != nil {
					b.conn.Close()
				}
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					return fmt.Errorf("failed to connect to page WebSocket: %w", err)
				}
				b.conn = conn
				b.wsEndpoint = wsURL
				return nil
			}
		}
	}
	return fmt.Errorf("no suitable page found")
}
func (b *Browser) SendCommandWithResponse(method string, params map[string]interface{}) (map[string]interface{}, error) {
	b.messageMutex.Lock()
	b.messageID++
	id := b.messageID
	b.messageMutex.Unlock()

	message := map[string]interface{}{
		"id":     id,
		"method": method,
		"params": params,
	}

	if b.conn == nil {
		if err := b.attachToPage(); err != nil {
			return nil, fmt.Errorf("failed to reconnect ws: %w", err)
		}
	}

	if err := b.conn.WriteJSON(message); err != nil {
		return nil, fmt.Errorf("failed to send ws message: %w", err)
	}

	for {
		_, data, err := b.conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("failed to read ws message: %w", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(data, &response); err != nil {
			continue
		}

		if responseID, ok := response["id"].(float64); ok && int(responseID) == id {
			return response, nil
		}
	}
}

func (b *Browser) SendCommandWithoutResponse(method string, params map[string]interface{}) error {
	b.messageMutex.Lock()
	b.messageID++
	id := b.messageID
	b.messageMutex.Unlock()

	message := map[string]interface{}{
		"id":     id,
		"method": method,
		"params": params,
	}

	if b.conn == nil {
		if err := b.attachToPage(); err != nil {
			return fmt.Errorf("failed to reconnect ws: %w", err)
		}
	}

	if err := b.conn.WriteJSON(message); err != nil {
		return fmt.Errorf("failed to send ws message: %w", err)
	}

	return nil
}

func (b *Browser) NewPage() (*page.Page, error) {
	if b.conn == nil {
		return nil, errors.New("ws connection not established, cannot create a new page")
	}
	return page.NewPage(b), nil
}

func (b *Browser) RedLight() error {
	if b.conn != nil {
		if err := b.conn.Close(); err != nil {
			return fmt.Errorf("error closing ws: %w", err)
		}
	}

	if b.cmd != nil && b.cmd.Process != nil {
		if err := b.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("error killing browser process: %w", err)
		} else {
			if err = b.cmd.Wait(); err != nil {
				return err
			}
		}
	}

	if b.userDataDir != "" {
		if err := os.RemoveAll(b.userDataDir); err != nil {
			return fmt.Errorf("error removing user data directory: %w", err)
		}
	}

	b.cancel()

	return nil
}
