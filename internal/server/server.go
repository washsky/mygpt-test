package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/washsky/mygpt-test/internal/appdata"
	"github.com/washsky/mygpt-test/internal/calculator"
	"github.com/washsky/mygpt-test/internal/filemanager"
)

type calculationRequest struct {
	A  float64 `json:"a"`
	B  float64 `json:"b"`
	Op string  `json:"op"`
}

type calculationResponse struct {
	A         float64 `json:"a"`
	B         float64 `json:"b"`
	Op        string  `json:"op"`
	Result    float64 `json:"result"`
	Version   string  `json:"version"`
}

func Start(addr, version, dataDir string) error {
	paths, err := appdata.Open(dataDir)
	if err != nil {
		return err
	}
	files, err := filemanager.NewStore(paths.Files)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	filemanager.RegisterRoutes(mux, files)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, page)
	})
	mux.HandleFunc("/api/calculate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var input calculationRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
		result, err := calculator.Calculate(input.A, input.B, input.Op)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(calculationResponse{
			A: input.A, B: input.B, Op: input.Op, Result: result, Version: version,
		})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, "{\"status\":\"ok\",\"version\":%q}\n", version)
	})

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	actualAddr := listener.Addr().String()
	if host, port, splitErr := net.SplitHostPort(actualAddr); splitErr == nil && (host == "" || host == "0.0.0.0" || host == "::") {
		actualAddr = net.JoinHostPort("127.0.0.1", port)
	}

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("mygpt-test %s is ready", version)
	log.Printf("data directory: %s", paths.Root)
	log.Printf("file manager: http://%s/files", actualAddr)
	log.Printf("Open in your browser: http://%s", actualAddr)
	log.Printf("API endpoint: http://%s/api/calculate", actualAddr)
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

const page = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>mygpt-test · 在线计算器</title>
<style>
:root{color-scheme:light;--ink:#15233b;--muted:#64748b;--accent:#2764e7;--line:#e1e8f2}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px;background:radial-gradient(circle at 15% 10%,#dceaff,transparent 34%),linear-gradient(145deg,#f5f8ff,#edf2fa);font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--ink)}
.card{width:min(100%,500px);padding:32px;border:1px solid #ffffffc9;border-radius:24px;background:#ffffffed;box-shadow:0 24px 70px #27446c1a}
.badge{display:inline-flex;align-items:center;gap:8px;padding:6px 11px;border-radius:999px;background:#edf4ff;color:#2458bd;font-size:13px;font-weight:650}
.dot{width:8px;height:8px;border-radius:50%;background:#38b980}
h1{margin:16px 0 4px;font-size:clamp(26px,6vw,36px);letter-spacing:-.04em}
.sub{margin:0 0 26px;color:var(--muted)}
.row{display:grid;grid-template-columns:1fr 88px 1fr;gap:10px}
input,select,button{width:100%;height:54px;border-radius:13px;font:inherit}
input,select{border:1px solid var(--line);background:white;padding:0 14px;color:var(--ink);outline:none}
input:focus,select:focus{border-color:#7da5fa;box-shadow:0 0 0 3px #2764e71b}
button{margin-top:14px;border:0;background:var(--accent);color:white;font-weight:700;cursor:pointer;box-shadow:0 8px 18px #2764e733}
button:hover{background:#1e53ca}
.result{min-height:84px;margin-top:18px;padding:16px;border:1px solid var(--line);border-radius:15px;background:#f8faff}
.result small{display:block;color:var(--muted)}
.value{font-size:28px;font-weight:750;overflow-wrap:anywhere}
.error{color:#bd2c3c}
footer{margin-top:22px;color:var(--muted);font-size:13px;text-align:center}
@media(max-width:420px){.card{padding:23px}.row{grid-template-columns:1fr 70px 1fr;gap:7px}input,select{padding:0 9px}}
</style>
</head>
<body>
<main class="card">
<div style="text-align:right;margin-bottom:12px"><a href="/files">文件管理 →</a></div>
<div class="badge"><span class="dot"></span>本地运行 · Go Web 示例</div>
<h1>简易计算器</h1>
<p class="sub">输入两个数字，选择运算方式，结果由本地 Go 服务计算。</p>
<form id="calc">
<div class="row">
<input id="a" type="number" step="any" value="12" aria-label="第一个数字" required>
<select id="op" aria-label="运算符"><option value="+">加法 ＋</option><option value="-">减法 −</option><option value="*">乘法 ×</option><option value="/">除法 ÷</option></select>
<input id="b" type="number" step="any" value="3" aria-label="第二个数字" required>
</div>
<button type="submit">计算结果</button>
</form>
<section class="result" aria-live="polite"><small>计算结果</small><div id="result" class="value">15</div></section>
<footer>mygpt-test · 版本 <span id="version">加载中</span></footer>
</main>
<script>
const form=document.querySelector("#calc"), result=document.querySelector("#result"), version=document.querySelector("#version");
async function calculate(event){event?.preventDefault();result.className="value";result.textContent="计算中…";try{const response=await fetch("/api/calculate",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({a:Number(document.querySelector("#a").value),b:Number(document.querySelector("#b").value),op:document.querySelector("#op").value})});if(!response.ok)throw new Error((await response.text()).trim());const data=await response.json();result.textContent=String(data.result);version.textContent=data.version}catch(error){result.className="value error";result.textContent=error.message}}
form.addEventListener("submit",calculate);calculate();
</script>
</body>
</html>`
