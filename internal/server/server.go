package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/washsky/mygpt-test/internal/appdata"
	"github.com/washsky/mygpt-test/internal/calculator"
	"github.com/washsky/mygpt-test/internal/filemanager"
	"github.com/washsky/mygpt-test/internal/forum"
 "github.com/washsky/mygpt-test/internal/ui"
 "github.com/washsky/mygpt-test/internal/live"
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

type expressionCalculationRequest struct {
	Expression string `json:"expression"`
}

type expressionCalculationResponse struct {
	Expression string  `json:"expression"`
	Result     float64 `json:"result"`
	Version    string  `json:"version"`
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
	board, err := forum.Open(paths.Database, files)
	if err != nil { return err }
	defer board.Close()
	token, err := board.AdminToken(paths.Config)
	if err != nil { return err }
	mux := http.NewServeMux()
 events,err := live.New(func() (live.Options,error) { s,err:=board.Settings();return live.Options{Realtime:s.EnableRealtime,Clipboard:s.EnableClipboard,ClipboardTTLMinutes:s.ClipboardTTLMinutes},err },paths.Database,files)
 if err!=nil{return err}
 defer events.Close()
 events.Register(mux)
	filemanager.RegisterRoutes(mux, files, board, token)
	(&forum.Handler{Store: board, Token: token}).Register(mux)
	mux.HandleFunc("GET /calculator", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, page)
	})
	mux.HandleFunc("POST /api/calculate-expression", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input expressionCalculationRequest
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			http.Error(w, "unexpected data after JSON request", http.StatusBadRequest)
			return
		}
		if len(input.Expression) > calculator.MaxExpressionLength {
			http.Error(w, fmt.Sprintf("expression cannot exceed %d bytes", calculator.MaxExpressionLength), http.StatusBadRequest)
			return
		}
		result, err := calculator.Evaluate(input.Expression)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(expressionCalculationResponse{
			Expression: input.Expression, Result: result, Version: version,
		})
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
		Handler:           sameOriginWrites(events.Wrap(mux)),
		ReadHeaderTimeout: 5 * time.Second,
 IdleTimeout: 90 * time.Second,
 MaxHeaderBytes: 1 << 20,
	}
	log.Printf("mygpt-test %s is ready", version)
	log.Printf("data directory: %s", paths.Root)
	log.Printf("file manager: http://%s/files", actualAddr)
	log.Printf("forum admin token file: %s/config/admin-token", paths.Root)
	log.Printf("Open in your browser: http://%s", actualAddr)
	log.Printf("API endpoint: http://%s/api/calculate", actualAddr)
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func sameOriginWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				parsed, err := url.Parse(origin)
				if err != nil || parsed.Host != r.Host || (parsed.Scheme != "http" && parsed.Scheme != "https") {
					http.Error(w, "cross-origin write rejected", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

const page = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#f3f6fb"><title>科学计算器 · mygpt-test</title>
<style>
:root{color-scheme:light;--bg:#f3f6fb;--panel:#fff;--soft:#f7f9fc;--ink:#172033;--muted:#758096;--line:#e5eaf2;--accent:#3867ed;--accent2:#2852cf;--tint:#eaf0ff;--key:#f2f5fa;--shadow:0 28px 80px #243b5d12}
:root[data-theme=dark]{color-scheme:dark;--bg:#111722;--panel:#1b2432;--soft:#202b3b;--ink:#eef3ff;--muted:#9ba8bd;--line:#344157;--accent:#7898ff;--accent2:#8aa5ff;--tint:#293957;--key:#263246;--shadow:0 28px 80px #0008}
*{box-sizing:border-box}body{margin:0;min-height:100vh;background:radial-gradient(ellipse at 16% 0%,var(--tint),transparent 38%),var(--bg);color:var(--ink);font:15px/1.55 system-ui,-apple-system,"Segoe UI",sans-serif}button,input{font:inherit}button{color:inherit}a{color:inherit;text-decoration:none}
.topbar{height:68px;max-width:1220px;margin:auto;padding:0 24px;display:flex;align-items:center;justify-content:space-between}.brand{display:flex;align-items:center;gap:10px;font-weight:750}.mark{width:34px;height:34px;display:grid;place-items:center;border-radius:11px;background:var(--accent);color:white;font-size:20px}.nav{display:flex;align-items:center;gap:10px;color:var(--muted);font-size:14px}.nav a{padding:8px 10px;border-radius:10px}.nav a:hover{background:var(--panel);color:var(--ink)}.theme{width:38px;height:38px;border:1px solid var(--line);border-radius:12px;background:var(--panel);cursor:pointer}
main{max-width:1220px;margin:32px auto 64px;padding:0 24px}.intro{margin-bottom:22px}.eyebrow{color:var(--accent2);font-size:12px;font-weight:750;letter-spacing:.08em}.dot{display:inline-block;width:7px;height:7px;margin-right:7px;border-radius:50%;background:#2bb980}h1{margin:8px 0;font-size:clamp(30px,5vw,42px);line-height:1.12;letter-spacing:-.05em}.intro p{margin:8px 0;color:var(--muted);max-width:720px}.layout{display:grid;grid-template-columns:minmax(0,1.65fr) minmax(270px,.7fr);gap:18px;align-items:start}.card{border:1px solid var(--line);border-radius:21px;background:var(--panel);box-shadow:var(--shadow)}
.workspace{padding:19px}.workspace-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:5px}.workspace-head h2{margin:0;font-size:17px}.count{color:var(--muted);font-size:12px}.add{padding:9px 13px;border:0;border-radius:11px;background:var(--accent);color:white;font-weight:700;cursor:pointer}.add:hover{background:var(--accent2)}.add:disabled{opacity:.5;cursor:not-allowed}.hint{margin:0 0 15px;color:var(--muted);font-size:12px}
.window-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,310px),1fr));gap:12px}.window-card{min-width:0;padding:14px;border:1px solid var(--line);border-radius:16px;background:var(--soft);transition:border-color .15s,box-shadow .15s}.window-card.active{border-color:var(--accent);box-shadow:0 0 0 2px color-mix(in srgb,var(--accent) 18%,transparent)}.window-head{display:flex;justify-content:space-between;align-items:center;gap:8px;margin-bottom:10px}.window-name{display:flex;align-items:center;gap:8px;font-size:13px;font-weight:750}.window-dot{width:8px;height:8px;border-radius:50%;background:var(--line)}.active .window-dot{background:var(--accent)}.window-actions{display:flex;gap:6px}.mini{padding:5px 8px;border:1px solid var(--line);border-radius:8px;background:var(--panel);color:var(--muted);font-size:11px;cursor:pointer}.mini:hover{color:var(--accent2);border-color:var(--accent)}.mini:disabled{opacity:.45;cursor:not-allowed}.window-label{display:block;margin-bottom:5px;color:var(--muted);font-size:11px}.window-input{width:100%;height:42px;padding:0 11px;border:1px solid var(--line);border-radius:10px;outline:0;background:var(--panel);color:var(--ink);font-size:16px}.window-input:focus{border-color:var(--accent);box-shadow:0 0 0 3px color-mix(in srgb,var(--accent) 16%,transparent)}.window-output{display:flex;justify-content:space-between;align-items:center;gap:8px;margin-top:10px;padding-top:8px;border-top:1px solid var(--line)}.window-preview{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--muted);font-size:11px}.window-result{font-size:22px;font-weight:750;font-variant-numeric:tabular-nums;overflow-wrap:anywhere}.window-status{min-height:18px;margin:6px 0 0;color:var(--muted);font-size:11px}.window-status.error{color:#d84b5a}.window-calculate{width:100%;height:37px;margin-top:8px;border:0;border-radius:9px;background:var(--tint);color:var(--accent2);font-weight:700;cursor:pointer}.window-calculate:hover{background:color-mix(in srgb,var(--accent) 18%,var(--tint))}.window-calculate:disabled{opacity:.55;cursor:wait}
.keys-wrap{margin-top:17px;padding-top:14px;border-top:1px solid var(--line)}.keys-title{display:flex;justify-content:space-between;align-items:center;margin-bottom:9px;color:var(--muted);font-size:12px}.keys-title strong{color:var(--ink);font-size:13px}.keys{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:7px}.key{height:43px;border:1px solid transparent;border-radius:10px;background:var(--key);font-size:15px;font-weight:650;cursor:pointer}.key:hover{border-color:var(--accent);background:var(--tint)}.key:active{transform:scale(.97)}.fn{color:var(--accent2);font-size:12px}.op{background:var(--tint);color:var(--accent2);font-size:18px}.danger{color:#c84e5b}.equals{grid-column:span 2;background:var(--accent);color:white;font-size:19px}.equals:hover{background:var(--accent2)}.global-status{min-height:22px;margin:9px 1px 0;color:var(--muted);font-size:12px}.global-status.error{color:#d84b5a}.api{margin:9px 1px 0;color:var(--muted);font-size:11px}.api code{padding:2px 5px;border-radius:5px;background:var(--soft);color:var(--accent2)}
.side{display:grid;gap:14px}.side-card{padding:18px}.side-title{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:0 0 12px;font-size:15px}.badge{padding:4px 8px;border-radius:99px;background:var(--tint);color:var(--accent2);font-size:10px;font-weight:700}.list{display:grid;gap:11px;margin:0;padding:0;list-style:none}.list li{display:flex;gap:9px;color:var(--muted);font-size:12px}.icon{width:24px;height:24px;flex:0 0 24px;display:grid;place-items:center;border-radius:8px;background:var(--tint);color:var(--accent2);font-weight:750}.tools{display:flex;gap:8px}.textbtn{border:0;background:none;color:var(--muted);font-size:11px;cursor:pointer}.textbtn:hover{color:var(--accent2)}.history{display:grid;gap:6px}.empty{margin:0;padding:10px 0;color:var(--muted);font-size:12px}.history-item{width:100%;display:flex;justify-content:space-between;gap:8px;padding:9px;border:1px solid transparent;border-radius:9px;background:var(--soft);text-align:left;cursor:pointer}.history-item:hover{border-color:var(--line);background:var(--tint)}.history-expr{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--muted);font-size:11px}.history-result{font-weight:700;font-variant-numeric:tabular-nums;white-space:nowrap}footer{margin-top:19px;text-align:center;color:var(--muted);font-size:11px}:focus-visible{outline:3px solid #3867ed77;outline-offset:2px}
@media(max-width:900px){main{margin-top:24px}.layout{grid-template-columns:1fr}.side{grid-template-columns:1fr 1fr}}
@media(max-width:600px){.topbar{height:60px;padding:0 13px}.nav{gap:2px;font-size:12px}.nav a{padding:7px}.brand{font-size:14px}.mark{width:30px;height:30px}main{margin:20px auto 42px;padding:0 12px}.intro{margin-bottom:16px}.workspace{padding:13px;border-radius:17px}.workspace-head{align-items:flex-start}.window-grid{grid-template-columns:1fr}.keys{gap:5px}.key{height:41px;border-radius:9px}.side{grid-template-columns:1fr}.window-result{font-size:20px}}
@media(prefers-reduced-motion:reduce){*,*::before,*::after{scroll-behavior:auto!important;transition:none!important}}
.layout>*,.side,.side-card,.history,.history-item,.window-output,.window-input{min-width:0;max-width:100%}.window-output{display:grid;grid-template-columns:minmax(0,1fr)}.window-preview{white-space:normal;overflow-wrap:anywhere}.window-result,.window-status,.history-expr,.history-result{min-width:0;overflow-wrap:anywhere;white-space:normal}.history-item{display:grid;grid-template-columns:minmax(0,1fr);cursor:default}.history-actions{display:flex;gap:8px;flex-wrap:wrap}.history-result{font-size:16px}.keys-title,.workspace-head{flex-wrap:wrap;gap:10px}.keypad-mode{padding:6px 10px;border:1px solid var(--line);border-radius:8px}.simple-keys{grid-template-columns:repeat(4,minmax(0,1fr))}.simple-keys .equals{grid-column:auto}.shared-row{padding:12px 0;border-top:1px solid var(--line);overflow-wrap:anywhere}.shared-row p{white-space:pre-wrap}.window-output output{user-select:all}
</style>` + ui.Head + `
</head>
<body>
` + ui.Nav + `
<main>
<section class="intro"><div class="eyebrow"><span class="dot"></span>工作空间 / 随手计算</div><h1>计算工具</h1><p>并排比较不同方案，一键把计算过程存入笔记，和资料放在一起。</p></section>
<div class="layout">
<section class="card workspace"><div class="workspace-head"><div><h2>计算窗口 <span id="window-count" class="count"></span> <span id="version" class="count">本地服务</span></h2></div><div class="tools"><button id="open-shared" class="mini" type="button">导入共享窗口</button><button id="add-window" class="add" type="button">＋ 添加窗口</button></div></div><p class="hint">每个窗口有独立表达式和结果；点击输入框或窗口即可选中，再使用下方按键。最多同时打开 4 个。</p>
<div id="windows" class="window-grid" aria-label="计算窗口"></div>
<div class="keys-wrap"><div class="keys-title"><strong>快捷按键</strong><label>布局 <select id="keypad-mode" class="keypad-mode"><option value="simple">简易</option><option value="scientific">科学</option></select></label></div><div id="simple-keys" class="keys simple-keys" aria-label="简易计算按键"></div><div id="keys" class="keys" aria-label="科学计算按键">
<button class="key danger" type="button" data-action="clear">AC</button><button class="key" type="button" data-action="backspace" aria-label="退格">⌫</button><button class="key" type="button" data-insert="(">(</button><button class="key" type="button" data-insert=")">)</button><button class="key fn" type="button" data-insert="%">mod</button><button class="key op" type="button" data-insert="/">÷</button>
<button class="key fn" type="button" data-fn="sin">sin</button><button class="key fn" type="button" data-fn="cos">cos</button><button class="key fn" type="button" data-fn="tan">tan</button><button class="key fn" type="button" data-fn="sqrt">√</button><button class="key fn" type="button" data-fn="ln">ln</button><button class="key fn" type="button" data-fn="log">log</button>
<button class="key" type="button" data-insert="7">7</button><button class="key" type="button" data-insert="8">8</button><button class="key" type="button" data-insert="9">9</button><button class="key fn" type="button" data-insert="^2">x²</button><button class="key fn" type="button" data-insert="^">xʸ</button><button class="key op" type="button" data-insert="*">×</button>
<button class="key" type="button" data-insert="4">4</button><button class="key" type="button" data-insert="5">5</button><button class="key" type="button" data-insert="6">6</button><button class="key fn" type="button" data-fn="abs">abs</button><button class="key fn" type="button" data-insert="pi">π</button><button class="key op" type="button" data-insert="-">−</button>
<button class="key" type="button" data-insert="1">1</button><button class="key" type="button" data-insert="2">2</button><button class="key" type="button" data-insert="3">3</button><button class="key fn" type="button" data-fn="floor">floor</button><button class="key fn" type="button" data-fn="ceil">ceil</button><button class="key op" type="button" data-insert="+">+</button>
<button class="key" type="button" data-insert="0">0</button><button class="key" type="button" data-insert="00">00</button><button class="key" type="button" data-insert=".">.</button><button class="key fn" type="button" data-insert="e">e</button><button class="key equals" type="button" data-action="calculate">计算 =</button>
</div><p id="global-status" class="global-status" role="status">按 Enter 计算当前窗口；Esc 清空当前表达式。</p><p class="hint">计算完成后，点击窗口中的“保存为笔记”即可带着表达式和结果回到论坛。</p></div>
</section>
<aside class="side">
<section class="card side-card"><h2 class="side-title">科学运算 <span class="badge">安全解析</span></h2><ul class="list">
<li><span class="icon">±</span><span><strong>四则与取余</strong><br>+、−、×、÷、mod</span></li><li><span class="icon">xʸ</span><span><strong>括号与幂</strong><br>支持嵌套括号和运算优先级</span></li><li><span class="icon">f</span><span><strong>科学函数</strong><br>sqrt、abs、sin、cos、tan、ln、log、floor、ceil</span></li><li><span class="icon">π</span><span><strong>常量</strong><br>pi、e；三角函数使用弧度</span></li>
</ul></section>
<section class="card side-card"><div class="side-title"><h2 style="margin:0;font-size:15px">最近计算</h2><div class="tools"><button id="clear-history" class="textbtn" type="button">清空</button></div></div><div id="history" class="history" aria-live="polite"></div></section>
</aside>
</div><footer>由当前服务计算 · 窗口与历史保存在当前浏览器 · 可按需共享窗口快照 · 版本 <span id="footer-version">读取中</span></footer>
</main>
<dialog id="shared-windows"><h2>共享的计算窗口</h2><p class="muted small">在窗口的“更多操作”中发送快照，再到另一浏览器导入。快照使用共享剪贴板的有效期；导入不会覆盖现有窗口。</p><div class="tools"><button id="refresh-shared" type="button" class="mini">刷新</button><button id="close-shared" type="button" class="mini">关闭</button></div><p id="shared-status" role="status"></p><div id="shared-list"></div></dialog>
<script>
const windowArea=document.querySelector("#windows"),globalStatus=document.querySelector("#global-status"),historyBox=document.querySelector("#history"),windowKey="mygpt.calculator.windows.v1",historyKey="mygpt.calculator.history.v1",maxWindows=4;
let calcWindows=[],activeId=1,historyItems=[];
function makeWindow(id){return{id,expression:"",result:"",display:"",status:"",error:false,busy:false}}
try{const saved=JSON.parse(localStorage.getItem(windowKey)||"[]");if(Array.isArray(saved))calcWindows=saved.filter(w=>w&&Number.isInteger(w.id)&&typeof w.expression==="string").slice(0,maxWindows).map(w=>({...makeWindow(w.id),expression:w.expression.slice(0,256),result:typeof w.result==="string"?w.result:"",display:typeof w.display==="string"?w.display:""}))}catch{}
if(calcWindows.length===0)calcWindows=[makeWindow(1),makeWindow(2)];
activeId=calcWindows[0].id;
try{const saved=JSON.parse(localStorage.getItem(historyKey)||"[]");if(Array.isArray(saved))historyItems=saved.filter(x=>x&&typeof x.expression==="string"&&typeof x.result==="string").slice(0,12)}catch{}
function findWindow(id){return calcWindows.find(w=>w.id===Number(id))}
function inputFor(id){return windowArea.querySelector('.window-input[data-window-id="'+id+'"]')}
function saveWindows(){try{localStorage.setItem(windowKey,JSON.stringify(calcWindows.map(({id,expression,result,display})=>({id,expression,result,display}))))}catch{}}
function format(value){const number=Number(value),size=Math.abs(number);if(size>=1e15||(size>0&&size<1e-6))return String(number);return new Intl.NumberFormat("zh-CN",{maximumSignificantDigits:15,useGrouping:false}).format(number)}
function say(message,error){globalStatus.textContent=message;globalStatus.classList.toggle("error",Boolean(error))}
function setActive(id){activeId=Number(id);windowArea.querySelectorAll(".window-card").forEach(card=>card.classList.toggle("active",card.dataset.windowId===String(activeId)))}
function makeButton(label,cls,action,id,title){const b=document.createElement("button");b.type="button";b.className=cls;b.textContent=label;b.dataset.action=action;b.dataset.windowId=String(id);if(title){b.title=title;b.setAttribute("aria-label",title)}return b}
function renderWindows(){
 windowArea.replaceChildren();
 for(const w of calcWindows){
  const card=document.createElement("article");card.className="window-card";card.dataset.windowId=String(w.id);
  const head=document.createElement("div");head.className="window-head";
  const name=document.createElement("span");name.className="window-name";const dot=document.createElement("span");dot.className="window-dot";name.append(dot,document.createTextNode("窗口 "+w.id));
  const actions=document.createElement("div");actions.className="window-actions";const copyExpression=makeButton("复制表达式","mini","copy-expression",w.id);const copyResult=makeButton("复制结果","mini","copy-result",w.id);copyResult.disabled=!w.result;actions.append(copyExpression,copyResult,makeButton("关闭","mini","close-window",w.id,"关闭窗口"));const note=makeButton("保存为笔记","mini","save-note",w.id);note.disabled=!w.result;actions.append(note,makeButton("共享窗口快照","mini","share-window",w.id));window.workspaceCompactActions(actions,0);head.append(name,actions);
  const label=document.createElement("label");label.className="window-label";label.htmlFor="expression-"+w.id;label.textContent="输入表达式";
  const input=document.createElement("input");input.id="expression-"+w.id;input.className="window-input";input.type="text";input.maxLength=256;input.autocomplete="off";input.autocapitalize="off";input.spellcheck=false;input.placeholder="例如 sqrt(81) + 2^3";input.value=w.expression;input.readOnly=w.busy;input.dataset.windowId=String(w.id);input.setAttribute("aria-label","计算窗口 "+w.id+" 的表达式");
  const outputLine=document.createElement("div");outputLine.className="window-output";
  const preview=document.createElement("span");preview.className="window-preview";preview.textContent=w.expression?w.expression+" =":"尚未计算";
  const result=document.createElement("output");result.className="window-result";result.textContent=w.display||"—";result.title=w.result;result.setAttribute("aria-live","polite");
  outputLine.append(preview,result);
  const status=document.createElement("p");status.className="window-status"+(w.error?" error":"");status.textContent=w.status||"结果会保留在此窗口。";
  const calculateButton=makeButton(w.busy?"计算中…":"计算","window-calculate","calculate-window",w.id);calculateButton.disabled=w.busy;
  card.append(head,label,input,outputLine,status,calculateButton);windowArea.append(card);
 }
 document.querySelector("#window-count").textContent=calcWindows.length+" / "+maxWindows;
 const add=document.querySelector("#add-window");add.disabled=calcWindows.length>=maxWindows;
 setActive(activeId);
}
function updateWindow(w){
 const card=windowArea.querySelector('.window-card[data-window-id="'+w.id+'"]');if(!card)return;
 card.querySelector(".window-preview").textContent=w.expression?w.expression+" =":"尚未计算";
 card.querySelector(".window-result").textContent=w.display||"—";
 const status=card.querySelector(".window-status");status.textContent=w.status||"结果会保留在此窗口。";status.classList.toggle("error",Boolean(w.error));
 const button=card.querySelector('[data-action="calculate-window"]');button.textContent=w.busy?"计算中…":"计算";button.disabled=w.busy;card.querySelector('[data-action="copy-result"]').disabled=!w.result;card.querySelector('[data-action="save-note"]').disabled=!w.result;
}
function renderHistory(){historyBox.replaceChildren();if(!historyItems.length){const p=document.createElement("p");p.className="empty";p.textContent="完成计算后会显示在这里。";historyBox.append(p);return}historyItems.forEach((item,index)=>{const row=document.createElement("div");row.className="history-item";const e=document.createElement("span");e.className="history-expr";e.textContent=item.expression;const r=document.createElement("span");r.className="history-result";r.textContent=format(item.result);r.title=item.result;const actions=document.createElement("div");actions.className="history-actions";for(const [action,label] of [["load","载入"],["copy","复制结果"],["copy-expression","复制表达式"]]){const b=document.createElement("button");b.type="button";b.className="mini";b.textContent=label;b.dataset.historyIndex=String(index);b.dataset.historyAction=action;actions.append(b)}row.append(e,r,actions);historyBox.append(row)})}
function saveHistory(){try{localStorage.setItem(historyKey,JSON.stringify(historyItems))}catch{}}
function insert(value){const input=inputFor(activeId);if(!input)return;if(input.readOnly)return;const start=input.selectionStart??input.value.length,end=input.selectionEnd??input.value.length;if(input.value.length-(end-start)+value.length>256){say("表达式最多 256 个字符。",true);return}input.setRangeText(value,start,end,"end");input.focus();input.dispatchEvent(new Event("input",{bubbles:true}))}
function clearActive(){const input=inputFor(activeId),w=findWindow(activeId);if(!input||!w||w.busy)return;input.value="";w.expression="";w.result="";w.display="";w.status="";w.error=false;updateWindow(w);saveWindows();input.focus();say("已清空窗口 "+w.id+"。")}
async function calculateWindow(id){
 const w=findWindow(id),input=inputFor(id);if(!w||!input||w.busy)return;const expression=input.value.trim();if(!expression){w.status="请输入表达式。";w.error=true;updateWindow(w);input.focus();return}
 w.expression=expression;w.status="正在本机计算…";w.error=false;w.busy=true;input.readOnly=true;updateWindow(w);
 try{
  const response=await fetch("/api/calculate-expression",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({expression})});
  if(!response.ok)throw new Error((await response.text()).trim()||"计算失败");
  const data=await response.json();w.result=String(data.result);w.display=format(data.result);w.status="计算完成";w.error=false;
  historyItems=historyItems.filter(item=>item.expression!==expression||item.result!==w.result);historyItems.unshift({expression,result:w.result,display:w.display,windowId:w.id});historyItems=historyItems.slice(0,12);saveHistory();renderHistory();
  const version=data.version||"本地服务";document.querySelector("#footer-version").textContent=version;document.querySelector("#version").textContent=version;say("窗口 "+w.id+" 计算完成。");
 }catch(error){w.result="";w.display="错误";w.status=error.message||"计算失败";w.error=true;say("窗口 "+w.id+"："+w.status,true)}
 finally{w.busy=false;const currentInput=inputFor(w.id);if(currentInput)currentInput.readOnly=false;updateWindow(w);saveWindows()}
}
async function copyText(text){let copied=false;try{if(navigator.clipboard&&window.isSecureContext)await navigator.clipboard.writeText(text);else{const area=document.createElement("textarea");area.value=text;area.style.position="fixed";area.style.opacity="0";document.body.append(area);area.select();const ok=document.execCommand("copy");area.remove();if(!ok)throw new Error("copy failed")}copied=true}catch{}const shared=await window.workspaceShareText?.(text);say(copied?(shared?"已复制并加入共享历史。":"已复制到剪贴板。"):(shared?"已加入共享历史；本机剪贴板未写入。":"复制失败，请手动选择结果复制。"),!copied&&!shared)}
windowArea.addEventListener("focusin",event=>{const card=event.target.closest(".window-card");if(card)setActive(card.dataset.windowId)});
windowArea.addEventListener("input",event=>{if(!event.target.matches(".window-input"))return;const w=findWindow(event.target.dataset.windowId);if(!w)return;w.expression=event.target.value;w.result="";w.display="";w.status="表达式已修改，请重新计算。";w.error=false;updateWindow(w);saveWindows()});
windowArea.addEventListener("keydown",event=>{const input=event.target.closest(".window-input");if(!input)return;if(event.key==="Enter"){event.preventDefault();calculateWindow(input.dataset.windowId)}else if(event.key==="Escape"){event.preventDefault();setActive(input.dataset.windowId);clearActive()}});
windowArea.addEventListener("click",event=>{
 const card=event.target.closest(".window-card");if(card)setActive(card.dataset.windowId);const button=event.target.closest("button[data-action]");if(!button)return;const id=Number(button.dataset.windowId),w=findWindow(id);
 if(button.dataset.action==="share-window"){shareWindow(w,button);return}
 if(button.dataset.action==="save-note"){if(!w?.result)return;try{sessionStorage.setItem("workspace-calculation-note",JSON.stringify({body:w.expression+" = "+w.result}));location.href="/#compose"}catch(_){say("浏览器无法暂存计算记录，请复制结果后粘贴到笔记。",true)}return}
 if(button.dataset.action==="calculate-window"){calculateWindow(id);return}
 if(button.dataset.action==="close-window"){if(w?.busy){say("请等待该窗口计算完成。",true);return}if(calcWindows.length<=1){say("至少保留一个计算窗口。",true);return}calcWindows=calcWindows.filter(item=>item.id!==id);if(activeId===id)activeId=calcWindows[0].id;saveWindows();renderWindows();say("已关闭窗口 "+id+"。");return}
 if(button.dataset.action==="copy-expression"){if(w?.expression)copyText(w.expression);else say("该窗口没有可复制的表达式。",true)}if(button.dataset.action==="copy-result"){if(w?.result)copyText(w.result);else say("该窗口还没有计算结果。",true)}
},true);
document.querySelector("#add-window").addEventListener("click",()=>{if(calcWindows.length>=maxWindows)return;const id=Math.max(0,...calcWindows.map(w=>w.id))+1;calcWindows.push(makeWindow(id));activeId=id;saveWindows();renderWindows();inputFor(id)?.focus();say("已添加窗口 "+id+"。")});
function handleKeyClick(event){const b=event.target.closest("button");if(!b)return;if(b.dataset.action==="clear"){clearActive();return}if(b.dataset.action==="backspace"){const input=inputFor(activeId);if(!input||input.readOnly)return;const start=input.selectionStart??input.value.length,end=input.selectionEnd??start;if(start===end&&start>0)input.setRangeText("",start-1,end,"end");else input.setRangeText("",start,end,"end");input.focus();input.dispatchEvent(new Event("input",{bubbles:true}));return}if(b.dataset.action==="calculate"){calculateWindow(activeId);return}if(b.dataset.fn){insert(b.dataset.fn+"(");return}if(b.dataset.insert)insert(b.dataset.insert)}
document.querySelector("#keys").addEventListener("click",handleKeyClick);document.querySelector("#simple-keys").addEventListener("click",handleKeyClick);
document.addEventListener("keydown",event=>{if(event.ctrlKey||event.metaKey||event.altKey||event.target.matches("input,textarea,button,select"))return;if(event.key==="Escape"){clearActive();return}if(event.key==="Backspace"){event.preventDefault();const input=inputFor(activeId);if(input&&!input.readOnly){const end=input.value.length;input.setRangeText("",Math.max(0,end-1),end,"end");input.focus();input.dispatchEvent(new Event("input",{bubbles:true}))}return}if(event.key==="Enter"){calculateWindow(activeId);return}if(event.key.length===1&&"0123456789.+-*/%^()".includes(event.key))insert(event.key)});
historyBox.addEventListener("click",event=>{const button=event.target.closest("button[data-history-index]");if(!button)return;const item=historyItems[Number(button.dataset.historyIndex)];if(!item)return;if(button.dataset.historyAction==="copy"){copyText(item.result);return}if(button.dataset.historyAction==="copy-expression"){copyText(item.expression);return}const w=findWindow(activeId);if(!w||w.busy){say("请等待当前窗口计算完成。",true);return}w.expression=item.expression;w.result=item.result;w.display=format(item.result);w.status="已载入历史结果；修改后可重新计算。";w.error=false;renderWindows();saveWindows();inputFor(w.id)?.focus();say("已载入当前窗口，没有新增历史记录。")});
document.querySelector("#clear-history").addEventListener("click",()=>{historyItems=[];saveHistory();renderHistory();say("计算记录已清空。")});
fetch("/healthz").then(r=>r.json()).then(data=>{document.querySelector("#version").textContent=data.version||"本地服务";document.querySelector("#footer-version").textContent=data.version||"本地服务"}).catch(()=>{document.querySelector("#version").textContent="本地服务";document.querySelector("#footer-version").textContent="本地服务"});
const simpleKeys=[["AC","action","clear"],["⌫","action","backspace"],["(","insert","("],[")","insert",")"],["7","insert","7"],["8","insert","8"],["9","insert","9"],["÷","insert","/"],["4","insert","4"],["5","insert","5"],["6","insert","6"],["×","insert","*"],["1","insert","1"],["2","insert","2"],["3","insert","3"],["−","insert","-"],["0","insert","0"],[".","insert","."],["=","action","calculate"],["+","insert","+"]];
for(const [label,kind,value] of simpleKeys){const button=document.createElement("button");button.type="button";button.className="key"+(value==="calculate"?" equals":"");button.textContent=label;button.dataset[kind]=value;if(value==="backspace")button.setAttribute("aria-label","退格");document.querySelector("#simple-keys").append(button)}
const modeSelect=document.querySelector("#keypad-mode");
try{modeSelect.value=localStorage.getItem("mygpt.calculator.keypad.v1")==="scientific"?"scientific":"simple"}catch{}
function applyKeypad(){const simple=modeSelect.value!=="scientific";document.querySelector("#simple-keys").hidden=!simple;document.querySelector("#keys").hidden=simple;try{localStorage.setItem("mygpt.calculator.keypad.v1",simple?"simple":"scientific")}catch{}}
modeSelect.addEventListener("change",applyKeypad);applyKeypad();
function parseSnapshot(text){if(typeof text!=="string")return null;const match=/^计算窗口：\n([\s\S]{1,256})\n结果：([^\n]*)$/.exec(text);if(!match)return null;const result=match[2];if(result!==""&&(!/^-?(?:\d+\.?\d*|\.\d+)(?:e[+-]?\d+)?$/i.test(result)||!Number.isFinite(Number(result))))return null;return {expression:match[1],result}}
async function shareWindow(w,button){if(!w?.expression.trim()){say("请先输入要共享的表达式。",true);return}if(w.busy){say("请等待计算完成后共享。",true);return}button.disabled=true;try{const response=await fetch("/api/clipboard",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({kind:"text",text:"计算窗口：\n"+w.expression+"\n结果："+w.result})});if(!response.ok)throw Error(response.status===403?"请先在论坛设置中开启共享剪贴板。":"发送失败，请检查连接后重试。");say("窗口快照已共享，可在另一浏览器点击“导入共享窗口”。")}catch(error){say(error.message,true)}finally{button.disabled=false}}
const sharedDialog=document.querySelector("#shared-windows"),sharedList=document.querySelector("#shared-list"),sharedStatus=document.querySelector("#shared-status");
let sharedRequest=0;
async function refreshShared(){const request=++sharedRequest;sharedStatus.textContent="正在读取共享窗口…";try{const response=await fetch("/api/clipboard");if(!response.ok)throw Error(response.status===403?"请先在论坛设置中开启共享剪贴板。":"读取失败，请重新刷新。");const data=await response.json();if(request!==sharedRequest)return;sharedList.replaceChildren();let count=0;for(const item of data.items||[]){const snapshot=item.kind==="text"?parseSnapshot(item.text):null;if(!snapshot)continue;count++;const row=document.createElement("div");row.className="shared-row";const text=document.createElement("p");text.textContent=snapshot.expression+(snapshot.result?"\n= "+format(snapshot.result):"\n尚未计算");const expiry=document.createElement("p");expiry.className="muted small";expiry.textContent="到期时间："+new Date(item.expires_at).toLocaleString();const button=document.createElement("button");button.type="button";button.className="mini";button.textContent="导入为新窗口";button.onclick=()=>{if(Date.parse(item.expires_at)<=Date.now()){sharedStatus.textContent="该快照已过期，请刷新。";return}if(calcWindows.length>=maxWindows){sharedStatus.textContent="已打开 4 个窗口，请先关闭一个再导入。";return}const id=Math.max(0,...calcWindows.map(w=>w.id))+1;calcWindows.push({...makeWindow(id),expression:snapshot.expression,result:snapshot.result,display:snapshot.result?format(snapshot.result):"",status:"已导入共享快照，可继续编辑或重新计算。"});activeId=id;saveWindows();renderWindows();sharedDialog.close();inputFor(id)?.focus();say("已导入共享窗口；原有窗口已保留。");};row.append(text,expiry,button);sharedList.append(row)}sharedStatus.textContent=count?"选择一份快照导入。":"暂无计算窗口快照，请先从另一浏览器发送。"}catch(error){if(request!==sharedRequest)return;sharedList.replaceChildren();sharedStatus.textContent=error.message}}
document.querySelector("#open-shared").onclick=()=>{sharedDialog.showModal();refreshShared()};
document.querySelector("#close-shared").onclick=()=>sharedDialog.close();
document.querySelector("#refresh-shared").onclick=refreshShared;
window.addEventListener("workspace-change",event=>{if(sharedDialog.open&&(!event.detail.path||event.detail.path.startsWith("/api/clipboard")))refreshShared()});

historyItems=historyItems.filter((item,index,items)=>items.findIndex(other=>other.expression===item.expression&&other.result===item.result)===index);saveHistory();
renderWindows();renderHistory();
</script>` + ui.Foot + `
</body>
</html>`
