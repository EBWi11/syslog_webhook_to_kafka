package common

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
)

type WebhookServer struct {
	listen  string
	path    string
	msgChan chan []byte
	server  *http.Server
	tls     *WebhookTLSConfig
}

func NewWebhook(listen, path string, msgChan chan []byte, tlsConfig *WebhookTLSConfig) (*WebhookServer, error) {
	if !strings.HasPrefix(listen, "http://") && !strings.HasPrefix(listen, "https://") {
		return nil, fmt.Errorf("listen address must start with http:// or https://")
	}

	// Extract host:port from the listen URL
	addr := strings.TrimPrefix(listen, "http://")
	addr = strings.TrimPrefix(addr, "https://")

	w := &WebhookServer{
		listen:  listen,
		path:    path,
		msgChan: msgChan,
		tls:     tlsConfig,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, w.handleWebhook)

	w.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Configure TLS if using HTTPS
	if strings.HasPrefix(listen, "https://") {
		if tlsConfig == nil || !tlsConfig.Enabled {
			return nil, fmt.Errorf("TLS configuration is required for HTTPS")
		}
		if tlsConfig.CertFile == "" || tlsConfig.KeyFile == "" {
			return nil, fmt.Errorf("both certificate and key files are required for HTTPS")
		}
	}

	return w, nil
}

func (w *WebhookServer) handleWebhook(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Validate JSON format using sonic
	var jsonData interface{}
	if err := sonic.Unmarshal(body, &jsonData); err != nil {
		http.Error(rw, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Send the original message to the channel
	w.msgChan <- body

	// Return success response
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("Message received successfully"))
}

func (w *WebhookServer) Run() error {
	if strings.HasPrefix(w.listen, "https://") {
		return w.server.ListenAndServeTLS(w.tls.CertFile, w.tls.KeyFile)
	}
	return w.server.ListenAndServe()
}

func (w *WebhookServer) Stop() error {
	return w.server.Close()
}

func (w *WebhookServer) ListenAddr() string {
	return w.server.Addr
}
