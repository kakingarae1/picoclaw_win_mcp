package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/kakingarae1/picoclaw/pkg/agent"
	"github.com/kakingarae1/picoclaw/pkg/config"
	"github.com/kakingarae1/picoclaw/pkg/singleton"
)

type Gateway struct {
	cfg   *config.Config
	a     *agent.Agent
	mu    sync.Mutex
	api   *http.Server
	ui    *http.Server
}

func New(cfg *config.Config) *Gateway { return &Gateway{cfg: cfg, a: agent.New(cfg)} }

func (g *Gateway) Start(ctx context.Context) error {
	if g.cfg.Security.SingleInstance {
		if err := singleton.Acquire("PicoClawGateway"); err != nil { return err }
	}
	apiAddr := fmt.Sprintf("127.0.0.1:%d", g.cfg.Gateway.Port)
	ln, err := net.Listen("tcp", apiAddr)
	if err != nil { return fmt.Errorf("bind %s: %w", apiAddr, err) }

	g.api = &http.Server{Handler: g.routes(), ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("[api]", apiAddr)
		_ = g.api.Serve(ln)
	}()

	if g.cfg.Gateway.WebUI {
		uiAddr := fmt.Sprintf("127.0.0.1:%d", g.cfg.Gateway.WebUIPort)
		if uiLn, err := net.Listen("tcp", uiAddr); err == nil {
			g.ui = &http.Server{Handler: g.uiRoutes(), ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second}
			wg.Add(1)
			go func() { defer wg.Done(); fmt.Println("[ui]", uiAddr); _ = g.ui.Serve(uiLn) }()
		}
	}

	<-ctx.Done()
	sh, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = g.api.Shutdown(sh)
	if g.ui != nil { _ = g.ui.Shutdown(sh) }
	wg.Wait()
	return nil
}

func (g *Gateway) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	m.HandleFunc("POST /chat", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Message string `json:"message"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
			http.Error(w, `{"error":"message required"}`, 400); return
		}
		g.mu.Lock(); defer g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		resp, err := g.a.Chat(r.Context(), req.Message)
		if err != nil {
			w.WriteHeader(500)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"response": resp})
	})
	m.HandleFunc("POST /reset", func(w http.ResponseWriter, _ *http.Request) {
		g.mu.Lock(); g.a.Reset(); g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
	})
	return cors(m)
}

func (g *Gateway) uiRoutes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html"); _, _ = w.Write([]byte(webUI))
	})
	m.Handle("/api/", http.StripPrefix("/api", g.routes()))
	return m
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o := r.Header.Get("Origin")
		if o == "http://localhost:18800" || o == "http://127.0.0.1:18800" {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == "OPTIONS" { w.WriteHeader(204); return }
		next.ServeHTTP(w, r)
	})
}

const webUI = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><title>PicoClaw</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,sans-serif;background:#0f0f0f;color:#e0e0e0;height:100vh;display:flex;flex-direction:column}
#hdr{padding:12px 20px;background:#1a1a1a;border-bottom:1px solid #333;display:flex;align-items:center;gap:10px}
#dot{width:8px;height:8px;border-radius:50%;background:#22c55e}
#msgs{flex:1;overflow-y:auto;padding:20px;display:flex;flex-direction:column;gap:12px}
.msg{max-width:80%;padding:10px 14px;border-radius:12px;line-height:1.5;white-space:pre-wrap}
.user{align-self:flex-end;background:#2563eb;color:#fff;border-radius:12px 12px 2px 12px}
.bot{align-self:flex-start;background:#1e1e1e;border:1px solid #333;border-radius:12px 12px 12px 2px}
#irow{padding:16px;background:#1a1a1a;border-top:1px solid #333;display:flex;gap:8px}
#inp{flex:1;padding:10px 14px;background:#2a2a2a;border:1px solid #444;border-radius:8px;color:#e0e0e0;font-size:1rem;outline:none}
#inp:focus{border-color:#2563eb}
button{padding:10px 18px;border:none;border-radius:8px;cursor:pointer;font-size:.9rem}
#snd{background:#2563eb;color:#fff}#snd:hover{background:#1d4ed8}#snd:disabled{background:#444;cursor:not-allowed}
#rst{background:#333;color:#aaa}#rst:hover{background:#444;color:#fff}
</style></head><body>
<div id="hdr"><div id="dot"></div><b style="color:#fff">PicoClaw</b><span style="margin-left:auto;font-size:.75rem;color:#555">Windows MCP Edition</span></div>
<div id="msgs"></div>
<div id="irow"><input id="inp" placeholder="Ask anything..." autocomplete="off"/><button id="rst">Reset</button><button id="snd">Send</button></div>
<script>
const A='http://127.0.0.1:18790',M=document.getElementById('msgs'),I=document.getElementById('inp'),S=document.getElementById('snd'),D=document.getElementById('dot');
function add(cls,txt){const d=document.createElement('div');d.className='msg '+cls;d.textContent=txt;M.appendChild(d);M.scrollTop=M.scrollHeight;return d;}
async function chat(){const m=I.value.trim();if(!m)return;I.value='';S.disabled=true;add('user',m);const t=add('bot','...');
try{const r=await fetch(A+'/chat',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({message:m})});const d=await r.json();t.remove();add('bot',d.error?'Error: '+d.error:d.response);}
catch(e){t.remove();D.style.background='#ef4444';add('bot','Cannot reach gateway.');}S.disabled=false;I.focus();}
document.getElementById('rst').onclick=async()=>{await fetch(A+'/reset',{method:'POST'});M.innerHTML='';add('bot','Reset.');};
S.onclick=chat;I.onkeydown=e=>{if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();chat();}};
fetch(A+'/health').then(()=>D.style.background='#22c55e').catch(()=>D.style.background='#ef4444');
</script></body></html>`
