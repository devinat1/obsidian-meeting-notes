// services/gateway/internal/proxy/proxy.go
package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Proxy struct {
	targetURL  string
	httpClient *http.Client
}

type NewParams struct {
	TargetURL string
}

func New(params NewParams) *Proxy {
	return &Proxy{
		targetURL: params.TargetURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *Proxy) Forward(w http.ResponseWriter, originalRequest *http.Request, targetPath string) {
	url := p.targetURL + targetPath
	if originalRequest.URL.RawQuery != "" {
		url += "?" + originalRequest.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(
		originalRequest.Context(),
		originalRequest.Method,
		url,
		originalRequest.Body,
	)
	if err != nil {
		log.Printf("Error creating proxy request: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"proxy error"}`), http.StatusBadGateway)
		return
	}

	for key, values := range originalRequest.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding request to %s: %v", url, err)
		http.Error(w, fmt.Sprintf(`{"error":"upstream service unavailable"}`), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
