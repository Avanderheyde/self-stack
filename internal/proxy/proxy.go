package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

type Proxy struct {
	mu     sync.RWMutex
	routes map[string]*httputil.ReverseProxy
}

func New() *Proxy {
	return &Proxy{routes: make(map[string]*httputil.ReverseProxy)}
}

func (p *Proxy) Register(appName string, hostPort int) error {
	target, err := url.Parse(fmt.Sprintf("http://localhost:%d", hostPort))
	if err != nil {
		return err
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	p.mu.Lock()
	p.routes[appName] = rp
	p.mu.Unlock()
	return nil
}

func (p *Proxy) Deregister(appName string) {
	p.mu.Lock()
	delete(p.routes, appName)
	p.mu.Unlock()
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	parts := strings.SplitN(host, ".", 2)
	appName := parts[0]

	p.mu.RLock()
	rp, ok := p.routes[appName]
	p.mu.RUnlock()

	if !ok {
		http.Error(w, fmt.Sprintf("app %q not found", appName), http.StatusNotFound)
		return
	}
	rp.ServeHTTP(w, r)
}

func (p *Proxy) ListRoutes() map[string]bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]bool, len(p.routes))
	for k := range p.routes {
		out[k] = true
	}
	return out
}
