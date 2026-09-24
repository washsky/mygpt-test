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
		Handler:           sameOriginWrites(mux),
		ReadHeaderTimeout: 5 * time.Second,
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
</style>
</head>
<body>
<header class="topbar"><a class="brand" href="/"><span class="mark">∑</span><span>mygpt-test</span></a><nav class="nav" aria-label="主导航"><a href="/">论坛</a><a href="/files">文件管理</a><button id="theme" class="theme" type="button" aria-label="切换深色模式">◐</button></nav></header>
<main>
<section class="intro"><div class="eyebrow"><span class="dot"></span>本机 Go 服务 · 无需联网</div><h1>多窗口科学计算器</h1><p>在同一页面打开多个独立计算窗口，并排计算、对比结果。支持括号、优先级、幂运算和科学函数；表达式由 Go 后端安全解析。</p></section>
<div class="layout">
<section class="card workspace"><div class="workspace-head"><div><h2>计算窗口 <span id="window-count" class="count"></span></h2></div><button id="add-window" class="add" type="button">＋ 添加窗口</button></div><p class="hint">每个窗口有独立表达式和结果；点击输入框或窗口即可选中，再使用下方按键。最多同时打开 4 个。</p>
<div id="windows" class="window-grid" aria-label="计算窗口"></div>
<div class="keys-wrap"><div class="keys-title"><strong>快捷按键</strong><span>点击窗口输入框决定写入位置</span></div><div id="keys" class="keys" aria-label="科学计算按键">
<button class="key danger" type="button" data-action="clear">AC</button><button class="key" type="button" data-action="backspace" aria-label="退格">⌫</button><button class="key" type="button" data-insert="(">(</button><button class="key" type="button" data-insert=")">)</button><button class="key fn" type="button" data-insert="%">mod</button><button class="key op" type="button" data-insert="/">÷</button>
<button class="key fn" type="button" data-fn="sin">sin</button><button class="key fn" type="button" data-fn="cos">cos</button><button class="key fn" type="button" data-fn="tan">tan</button><button class="key fn" type="button" data-fn="sqrt">√</button><button class="key fn" type="button" data-fn="ln">ln</button><button class="key fn" type="button" data-fn="log">log</button>
<button class="key" type="button" data-insert="7">7</button><button class="key" type="button" data-insert="8">8</button><button class="key" type="button" data-insert="9">9</button><button class="key fn" type="button" data-insert="^2">x²</button><button class="key fn" type="button" data-insert="^">xʸ</button><button class="key op" type="button" data-insert="*">×</button>
<button class="key" type="button" data-insert="4">4</button><button class="key" type="button" data-insert="5">5</button><button class="key" type="button" data-insert="6">6</button><button class="key fn" type="button" data-fn="abs">abs</button><button class="key fn" type="button" data-insert="pi">π</button><button class="key op" type="button" data-insert="-">−</button>
<button class="key" type="button" data-insert="1">1</button><button class="key" type="button" data-insert="2">2</button><button class="key" type="button" data-insert="3">3</button><button class="key fn" type="button" data-fn="floor">floor</button><button class="key fn" type="button" data-fn="ceil">ceil</button><button class="key op" type="button" data-insert="+">+</button>
<button class="key" type="button" data-insert="0">0</button><button class="key" type="button" data-insert="00">00</button><button class="key" type="button" data-insert=".">.</button><button class="key fn" type="button" data-insert="e">e</button><button class="key equals" type="button" data-action="calculate">计算 =</button>
</div><p id="global-status" class="global-status" role="status">按 Enter 计算当前窗口；Esc 清空当前表达式。</p><p class="api">旧接口 <code>POST /api/calculate</code> 仍兼容 · 表达式接口 <code>POST /api/calculate-expression</code></p></div>
</section>
<aside class="side">
<section class="card side-card"><h2 class="side-title">科学运算 <span class="badge">安全解析</span></h2><ul class="list">
<li><span class="icon">±</span><span><strong>四则与取余</strong><br>+、−、×、÷、mod</span></li><li><span class="icon">xʸ</span><span><strong>括号与幂</strong><br>支持嵌套括号和运算优先级</span></li><li><span class="icon">f</span><span><strong>科学函数</strong><br>sqrt、abs、sin、cos、tan、ln、log、floor、ceil</span></li><li><span class="icon">π</span><span><strong>常量</strong><br>pi、e；三角函数使用弧度</span></li>
</ul></section>
<section class="card side-card"><div class="side-title"><h2 style="margin:0;font-size:15px">最近计算</h2><div class="tools"><button id="clear-history" class="textbtn" type="button">清空</button></div></div><div id="history" class="history" aria-live="polite"></div></section>
</aside>
</div><footer>结果在本机计算 · 窗口与历史保存在当前浏览器 · 版本 <span id="footer-version">读取中</span></footer>
</main>
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
function format(value){return new Intl.NumberFormat("zh-CN",{maximumSignificantDigits:15,useGrouping:false}).format(Number(value))}
function say(message,error){globalStatus.textContent=message;globalStatus.classList.toggle("error",Boolean(error))}
function setActive(id){activeId=Number(id);windowArea.querySelectorAll(".window-card").forEach(card=>card.classList.toggle("active",card.dataset.windowId===String(activeId)))}
function makeButton(label,cls,action,id,title){const b=document.createElement("button");b.type="button";b.className=cls;b.textContent=label;b.dataset.action=action;b.dataset.windowId=String(id);if(title){b.title=title;b.setAttribute("aria-label",title)}return b}
function renderWindows(){
 windowArea.replaceChildren();
 for(const w of calcWindows){
  const card=document.createElement("article");card.className="window-card";card.dataset.windowId=String(w.id);
  const head=document.createElement("div");head.className="window-head";
  const name=document.createElement("span");name.className="window-name";const dot=document.createElement("span");dot.className="window-dot";name.append(dot,document.createTextNode("窗口 "+w.id));
  const actions=document.createElement("div");actions.className="window-actions";const copyExpression=makeButton("复制表达式","mini","copy-expression",w.id);const copyResult=makeButton("复制结果","mini","copy-result",w.id);copyResult.disabled=!w.result;actions.append(copyExpression,copyResult,makeButton("关闭","mini","close-window",w.id,"关闭窗口"));head.append(name,actions);
  const label=document.createElement("label");label.className="window-label";label.htmlFor="expression-"+w.id;label.textContent="输入表达式";
  const input=document.createElement("input");input.id="expression-"+w.id;input.className="window-input";input.type="text";input.autocomplete="off";input.autocapitalize="off";input.spellcheck=false;input.placeholder="例如 sqrt(81) + 2^3";input.value=w.expression;input.dataset.windowId=String(w.id);input.setAttribute("aria-label","计算窗口 "+w.id+" 的表达式");
  const outputLine=document.createElement("div");outputLine.className="window-output";
  const preview=document.createElement("span");preview.className="window-preview";preview.textContent=w.expression?w.expression+" =":"尚未计算";
  const result=document.createElement("output");result.className="window-result";result.textContent=w.display||"—";result.setAttribute("aria-live","polite");
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
 const button=card.querySelector('[data-action="calculate-window"]');button.textContent=w.busy?"计算中…":"计算";button.disabled=w.busy;card.querySelector('[data-action="copy-result"]').disabled=!w.result;
}
function renderHistory(){historyBox.replaceChildren();if(!historyItems.length){const p=document.createElement("p");p.className="empty";p.textContent="完成计算后会显示在这里。";historyBox.append(p);return}for(const item of historyItems){const b=document.createElement("button");b.type="button";b.className="history-item";b.dataset.expression=item.expression;b.dataset.windowId=String(item.windowId);const e=document.createElement("span");e.className="history-expr";e.textContent=item.expression;const r=document.createElement("span");r.className="history-result";r.textContent=item.display||item.result;b.append(e,r);historyBox.append(b)}}
function saveHistory(){try{localStorage.setItem(historyKey,JSON.stringify(historyItems))}catch{}}
function insert(value){const input=inputFor(activeId);if(!input)return;const start=input.selectionStart??input.value.length,end=input.selectionEnd??input.value.length;input.setRangeText(value,start,end,"end");input.focus();input.dispatchEvent(new Event("input",{bubbles:true}))}
function clearActive(){const input=inputFor(activeId),w=findWindow(activeId);if(!input||!w)return;input.value="";w.expression="";w.result="";w.display="";w.status="";w.error=false;updateWindow(w);saveWindows();input.focus();say("已清空窗口 "+w.id+"。")}
async function calculateWindow(id){
 const w=findWindow(id),input=inputFor(id);if(!w||!input||w.busy)return;const expression=input.value.trim();if(!expression){w.status="请输入表达式。";w.error=true;updateWindow(w);input.focus();return}
 w.expression=expression;w.status="正在本机计算…";w.error=false;w.busy=true;input.readOnly=true;updateWindow(w);
 try{
  const response=await fetch("/api/calculate-expression",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({expression})});
  if(!response.ok)throw new Error((await response.text()).trim()||"计算失败");
  const data=await response.json();w.result=String(data.result);w.display=format(data.result);w.status="计算完成";w.error=false;
  historyItems.unshift({expression,result:w.result,display:w.display,windowId:w.id});historyItems=historyItems.slice(0,12);saveHistory();renderHistory();
  const version=data.version||"本地服务";document.querySelector("#footer-version").textContent=version;document.querySelector("#version").textContent=version;say("窗口 "+w.id+" 计算完成。");
 }catch(error){w.result="";w.display="错误";w.status=error.message||"计算失败";w.error=true;say("窗口 "+w.id+"："+w.status,true)}
 finally{w.busy=false;input.readOnly=false;updateWindow(w);saveWindows()}
}
async function copyText(text){try{if(navigator.clipboard&&window.isSecureContext)await navigator.clipboard.writeText(text);else{const area=document.createElement("textarea");area.value=text;area.style.position="fixed";area.style.opacity="0";document.body.append(area);area.select();const ok=document.execCommand("copy");area.remove();if(!ok)throw new Error("copy failed")}say("已复制到剪贴板。")}catch{say("复制失败，请检查浏览器剪贴板权限。",true)}}
windowArea.addEventListener("focusin",event=>{const card=event.target.closest(".window-card");if(card)setActive(card.dataset.windowId)});
windowArea.addEventListener("input",event=>{if(!event.target.matches(".window-input"))return;const w=findWindow(event.target.dataset.windowId);if(!w)return;w.expression=event.target.value;w.result="";w.display="";w.status="表达式已修改，请重新计算。";w.error=false;updateWindow(w);saveWindows()});
windowArea.addEventListener("keydown",event=>{const input=event.target.closest(".window-input");if(!input)return;if(event.key==="Enter"){event.preventDefault();calculateWindow(input.dataset.windowId)}else if(event.key==="Escape"){event.preventDefault();setActive(input.dataset.windowId);clearActive()}});
windowArea.addEventListener("click",event=>{
 const card=event.target.closest(".window-card");if(card)setActive(card.dataset.windowId);const button=event.target.closest("button[data-action]");if(!button)return;const id=Number(button.dataset.windowId),w=findWindow(id);
 if(button.dataset.action==="calculate-window"){calculateWindow(id);return}
 if(button.dataset.action==="close-window"){if(calcWindows.length<=1){say("至少保留一个计算窗口。",true);return}calcWindows=calcWindows.filter(item=>item.id!==id);if(activeId===id)activeId=calcWindows[0].id;saveWindows();renderWindows();say("已关闭窗口 "+id+"。");return}
 if(button.dataset.action==="copy-expression"){if(w?.expression)copyText(w.expression);else say("该窗口没有可复制的表达式。",true)}if(button.dataset.action==="copy-result"){if(w?.result)copyText(w.result);else say("该窗口还没有计算结果。",true)}
});
document.querySelector("#add-window").addEventListener("click",()=>{if(calcWindows.length>=maxWindows)return;const id=Math.max(0,...calcWindows.map(w=>w.id))+1;calcWindows.push(makeWindow(id));activeId=id;saveWindows();renderWindows();inputFor(id)?.focus();say("已添加窗口 "+id+"。")});
document.querySelector("#keys").addEventListener("click",event=>{const b=event.target.closest("button");if(!b)return;if(b.dataset.action==="clear"){clearActive();return}if(b.dataset.action==="backspace"){const input=inputFor(activeId);if(!input)return;const start=input.selectionStart??input.value.length,end=input.selectionEnd??start;if(start===end&&start>0)input.setRangeText("",start-1,end,"end");else input.setRangeText("",start,end,"end");input.focus();input.dispatchEvent(new Event("input",{bubbles:true}));return}if(b.dataset.action==="calculate"){calculateWindow(activeId);return}if(b.dataset.fn){insert(b.dataset.fn+"(");return}if(b.dataset.insert)insert(b.dataset.insert)});
document.addEventListener("keydown",event=>{if(event.ctrlKey||event.metaKey||event.altKey||event.target.matches("input,textarea,button,select"))return;if(event.key==="Escape"){clearActive();return}if(event.key==="Backspace"){event.preventDefault();const input=inputFor(activeId);if(input){const end=input.value.length;input.setRangeText("",Math.max(0,end-1),end,"end");input.focus();input.dispatchEvent(new Event("input",{bubbles:true}))}return}if(event.key==="Enter"){calculateWindow(activeId);return}if(event.key.length===1&&"0123456789.+-*/%^()".includes(event.key))insert(event.key)});
historyBox.addEventListener("click",event=>{const button=event.target.closest("[data-expression]");if(!button)return;const w=findWindow(button.dataset.windowId)||findWindow(activeId);if(!w)return;w.expression=button.dataset.expression;w.result="";w.display="";w.status="已从历史载入，请重新计算。";w.error=false;activeId=w.id;renderWindows();saveWindows();inputFor(w.id)?.focus();calculateWindow(w.id)});
document.querySelector("#clear-history").addEventListener("click",()=>{historyItems=[];saveHistory();renderHistory();say("计算记录已清空。")});
const themeButton=document.querySelector("#theme");try{if(localStorage.getItem("mygpt.calculator.theme")==="dark")document.documentElement.dataset.theme="dark"}catch{}
themeButton.addEventListener("click",()=>{const dark=document.documentElement.dataset.theme!=="dark";if(dark)document.documentElement.dataset.theme="dark";else delete document.documentElement.dataset.theme;try{localStorage.setItem("mygpt.calculator.theme",dark?"dark":"light")}catch{}});
fetch("/healthz").then(r=>r.json()).then(data=>{document.querySelector("#version").textContent=data.version||"本地服务";document.querySelector("#footer-version").textContent=data.version||"本地服务"}).catch(()=>{document.querySelector("#version").textContent="本地服务";document.querySelector("#footer-version").textContent="本地服务"});
renderWindows();renderHistory();
</script>
</body>
</html>`
