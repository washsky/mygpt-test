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
.topbar{height:68px;max-width:1180px;margin:auto;padding:0 24px;display:flex;align-items:center;justify-content:space-between}.brand{display:flex;align-items:center;gap:10px;font-weight:750}.mark{width:34px;height:34px;display:grid;place-items:center;border-radius:11px;background:var(--accent);color:white;font-size:20px}.nav{display:flex;align-items:center;gap:10px;color:var(--muted);font-size:14px}.nav a{padding:8px 10px;border-radius:10px}.nav a:hover{background:var(--panel);color:var(--ink)}.theme{width:38px;height:38px;border:1px solid var(--line);border-radius:12px;background:var(--panel);cursor:pointer}
main{max-width:1120px;margin:36px auto 64px;padding:0 24px}.intro{margin-bottom:24px}.eyebrow{color:var(--accent2);font-size:12px;font-weight:750;letter-spacing:.08em}.dot{display:inline-block;width:7px;height:7px;margin-right:7px;border-radius:50%;background:#2bb980}h1{margin:8px 0;font-size:clamp(30px,5vw,43px);line-height:1.12;letter-spacing:-.05em}.intro p{margin:8px 0;color:var(--muted);max-width:680px}.layout{display:grid;grid-template-columns:minmax(0,1.4fr) minmax(270px,.75fr);gap:20px;align-items:start}.card{border:1px solid var(--line);border-radius:22px;background:var(--panel);box-shadow:var(--shadow)}.calc{padding:20px}.heading{display:flex;justify-content:space-between;align-items:center;margin:0 0 15px}.heading h2{margin:0;font-size:16px}.badge{padding:5px 9px;border-radius:99px;background:var(--tint);color:var(--accent2);font-size:11px;font-weight:700}
.display{padding:14px 16px 16px;border:1px solid var(--line);border-radius:16px;background:var(--soft);margin-bottom:15px}.label{display:flex;justify-content:space-between;color:var(--muted);font-size:12px}.input{width:100%;height:49px;margin-top:4px;padding:0;border:0;outline:0;background:transparent;color:var(--ink);font-size:clamp(22px,4vw,30px);font-weight:600}.input::placeholder{color:var(--muted)}.answer-line{display:flex;justify-content:space-between;align-items:center;gap:10px;border-top:1px solid var(--line);padding-top:10px}.preview{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--muted);font-size:12px}.answer{display:flex;align-items:center;gap:8px}.result{font-size:25px;font-weight:750;letter-spacing:-.03em;font-variant-numeric:tabular-nums;overflow-wrap:anywhere}.copy{width:34px;height:34px;border:1px solid var(--line);border-radius:10px;background:var(--panel);cursor:pointer}.copy:disabled{opacity:.45;cursor:not-allowed}
.keys{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:8px}.key{height:49px;border:1px solid transparent;border-radius:12px;background:var(--key);font-size:16px;font-weight:650;cursor:pointer}.key:hover,.copy:hover{border-color:var(--accent);background:var(--tint)}.key:active{transform:scale(.97)}.fn{color:var(--accent2);font-size:13px}.op{background:var(--tint);color:var(--accent2);font-size:19px}.danger{color:#c84e5b}.equals{grid-column:span 2;background:var(--accent);color:white;font-size:20px}.equals:hover{background:var(--accent2)}.status{min-height:22px;margin:11px 2px 0;color:var(--muted);font-size:12px}.status.error{color:#d84b5a}.api{margin:12px 2px 0;color:var(--muted);font-size:12px}.api code{padding:2px 5px;border-radius:5px;background:var(--soft);color:var(--accent2)}
.side{display:grid;gap:15px}.side-card{padding:19px}.side-title{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:0 0 13px;font-size:15px}.list{display:grid;gap:12px;margin:0;padding:0;list-style:none}.list li{display:flex;gap:10px;color:var(--muted);font-size:13px}.icon{width:25px;height:25px;flex:0 0 25px;display:grid;place-items:center;border-radius:8px;background:var(--tint);color:var(--accent2);font-weight:750}.tools{display:flex;gap:8px}.textbtn{border:0;background:none;color:var(--muted);font-size:12px;cursor:pointer}.textbtn:hover{color:var(--accent2)}.history{display:grid;gap:6px}.empty{margin:0;padding:12px 0;color:var(--muted);font-size:13px}.item{width:100%;display:flex;justify-content:space-between;gap:8px;padding:10px;border:1px solid transparent;border-radius:10px;background:var(--soft);text-align:left;cursor:pointer}.item:hover{border-color:var(--line);background:var(--tint)}.item-expr{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--muted);font-size:12px}.item-value{font-weight:700;font-variant-numeric:tabular-nums;white-space:nowrap}footer{margin-top:21px;text-align:center;color:var(--muted);font-size:12px}:focus-visible{outline:3px solid #3867ed77;outline-offset:2px}
@media(max-width:800px){main{margin-top:25px}.layout{grid-template-columns:1fr}.side{grid-template-columns:1fr 1fr}}
@media(max-width:560px){.topbar{height:60px;padding:0 14px}.nav{gap:2px;font-size:12px}.nav a{padding:7px}.brand{font-size:14px}.mark{width:30px;height:30px}main{margin:20px auto 42px;padding:0 13px}.intro{margin-bottom:18px}.calc{padding:14px;border-radius:18px}.keys{gap:6px}.key{height:46px;border-radius:10px}.side{grid-template-columns:1fr}.result{font-size:22px}.display{padding:12px}}
@media(prefers-reduced-motion:reduce){*,*::before,*::after{scroll-behavior:auto!important;transition:none!important}}
</style>
</head>
<body>
<header class="topbar"><a class="brand" href="/"><span class="mark">∑</span><span>mygpt-test</span></a><nav class="nav" aria-label="主导航"><a href="/">论坛</a><a href="/files">文件管理</a><button id="theme" class="theme" type="button" aria-label="切换深色模式">◐</button></nav></header>
<main>
<section class="intro"><div class="eyebrow"><span class="dot"></span>本机 Go 服务 · 无需联网</div><h1>科学计算器</h1><p>支持括号、运算优先级、幂运算和常用科学函数。表达式由 Go 后端安全解析，不执行用户输入的代码。</p></section>
<div class="layout">
<section class="card calc" aria-label="计算器"><div class="heading"><h2>计算表达式</h2><span class="badge">Rad · 弧度制</span></div>
<div class="display"><div class="label"><label for="expression">表达式</label><span id="version">本地服务</span></div><input id="expression" class="input" type="text" inputmode="decimal" autocomplete="off" autocapitalize="off" spellcheck="false" placeholder="例如 2 * (3 + 4)" aria-describedby="status"><div class="answer-line"><span id="preview" class="preview">输入表达式后按 Enter 或点击 =</span><div class="answer"><output id="result" class="result" aria-live="polite">0</output><button id="copy-result" class="copy" type="button" aria-label="复制结果" title="复制结果" disabled>⧉</button></div></div></div>
<div id="keys" class="keys" aria-label="计算器按键">
<button class="key danger" type="button" data-action="clear" aria-label="清除">AC</button><button class="key" type="button" data-action="backspace" aria-label="退格">⌫</button><button class="key" type="button" data-insert="(">(</button><button class="key" type="button" data-insert=")">)</button><button class="key fn" type="button" data-insert="%">mod</button><button class="key op" type="button" data-insert="/">÷</button>
<button class="key fn" type="button" data-fn="sin">sin</button><button class="key fn" type="button" data-fn="cos">cos</button><button class="key fn" type="button" data-fn="tan">tan</button><button class="key fn" type="button" data-fn="sqrt">√</button><button class="key fn" type="button" data-fn="ln">ln</button><button class="key fn" type="button" data-fn="log">log</button>
<button class="key" type="button" data-insert="7">7</button><button class="key" type="button" data-insert="8">8</button><button class="key" type="button" data-insert="9">9</button><button class="key fn" type="button" data-insert="^2">x²</button><button class="key fn" type="button" data-insert="^">xʸ</button><button class="key op" type="button" data-insert="*">×</button>
<button class="key" type="button" data-insert="4">4</button><button class="key" type="button" data-insert="5">5</button><button class="key" type="button" data-insert="6">6</button><button class="key fn" type="button" data-fn="abs">abs</button><button class="key fn" type="button" data-insert="pi">π</button><button class="key op" type="button" data-insert="-">−</button>
<button class="key" type="button" data-insert="1">1</button><button class="key" type="button" data-insert="2">2</button><button class="key" type="button" data-insert="3">3</button><button class="key fn" type="button" data-fn="floor">floor</button><button class="key fn" type="button" data-fn="ceil">ceil</button><button class="key op" type="button" data-insert="+">+</button>
<button class="key" type="button" data-insert="0">0</button><button class="key" type="button" data-insert="00">00</button><button class="key" type="button" data-insert=".">.</button><button class="key fn" type="button" data-insert="e">e</button><button class="key equals" type="button" data-action="calculate">=</button>
</div><p id="status" class="status" role="status">可用键盘输入；Enter 计算，Esc 清空。</p><p class="api">兼容 <code>POST /api/calculate</code> · 表达式接口 <code>POST /api/calculate-expression</code></p></section>
<aside class="side">
<section class="card side-card"><h2 class="side-title">运算说明 <span class="badge">安全解析</span></h2><ul class="list">
<li><span class="icon">±</span><span><strong>四则与取余</strong><br>+、−、×、÷、mod</span></li><li><span class="icon">xʸ</span><span><strong>括号与幂</strong><br>支持嵌套括号和常规优先级</span></li><li><span class="icon">f</span><span><strong>科学函数</strong><br>sqrt、abs、sin、cos、tan、ln、log、floor、ceil</span></li><li><span class="icon">π</span><span><strong>常量</strong><br>pi、e；三角函数使用弧度</span></li>
</ul></section>
<section class="card side-card"><div class="side-title"><h2 style="margin:0;font-size:15px">最近计算</h2><div class="tools"><button id="copy-expression" class="textbtn" type="button">复制表达式</button><button id="clear-history" class="textbtn" type="button">清空</button></div></div><div id="history" class="history" aria-live="polite"></div></section>
</aside>
</div><footer>结果在本机计算 · 历史仅保存在此浏览器 · 版本 <span id="footer-version">读取中</span></footer>
</main>
<script>
const input=document.querySelector("#expression"),output=document.querySelector("#result"),preview=document.querySelector("#preview"),statusLine=document.querySelector("#status"),keys=document.querySelector("#keys"),historyBox=document.querySelector("#history"),copyButton=document.querySelector("#copy-result"),historyKey="mygpt.calculator.history.v1";
let lastResult="",items=[];
try{const saved=JSON.parse(localStorage.getItem(historyKey)||"[]");if(Array.isArray(saved))items=saved.filter(x=>x&&typeof x.expression==="string"&&typeof x.result==="string").slice(0,12)}catch{}
function format(value){return new Intl.NumberFormat("zh-CN",{maximumSignificantDigits:15,useGrouping:false}).format(Number(value))}
function say(message,error){statusLine.textContent=message;statusLine.classList.toggle("error",Boolean(error))}
function save(){try{localStorage.setItem(historyKey,JSON.stringify(items))}catch{}}
function renderHistory(){historyBox.replaceChildren();if(!items.length){const p=document.createElement("p");p.className="empty";p.textContent="计算记录保存在当前浏览器。";historyBox.append(p);return}for(const item of items){const b=document.createElement("button");b.type="button";b.className="item";b.dataset.expression=item.expression;const e=document.createElement("span");e.className="item-expr";e.textContent=item.expression;const v=document.createElement("span");v.className="item-value";v.textContent=item.display||item.result;b.append(e,v);historyBox.append(b)}}
function insert(value){const start=input.selectionStart??input.value.length,end=input.selectionEnd??input.value.length;input.setRangeText(value,start,end,"end");input.focus();say("表达式已更新。")}
function clearExpression(){input.value="";preview.textContent="输入表达式后按 Enter 或点击 =";output.textContent="0";lastResult="";copyButton.disabled=true;say("已清空。");input.focus()}
async function calculate(){const expression=input.value.trim();if(!expression){say("请输入表达式。",true);input.focus();return}say("正在本机计算…");preview.textContent=expression+" =";const button=keys.querySelector('[data-action="calculate"]');button.disabled=true;input.readOnly=true;try{const response=await fetch("/api/calculate-expression",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({expression})});if(!response.ok)throw new Error((await response.text()).trim()||"计算失败");const data=await response.json();lastResult=String(data.result);output.textContent=format(data.result);copyButton.disabled=false;const version=data.version||"本地服务";document.querySelector("#version").textContent=version;document.querySelector("#footer-version").textContent=version;items.unshift({expression,result:lastResult,display:format(data.result)});items=items.slice(0,12);save();renderHistory();say("计算完成。")}catch(error){say(error.message||"计算失败。",true)}finally{button.disabled=false;input.readOnly=false}}
async function copyText(text){try{if(navigator.clipboard&&window.isSecureContext)await navigator.clipboard.writeText(text);else{const area=document.createElement("textarea");area.value=text;area.style.position="fixed";area.style.opacity="0";document.body.append(area);area.select();const ok=document.execCommand("copy");area.remove();if(!ok)throw new Error("copy failed")}say("已复制到剪贴板。")}catch{say("复制失败，请检查浏览器剪贴板权限。",true)}}
keys.addEventListener("click",event=>{const b=event.target.closest("button");if(!b||b.disabled)return;if(b.dataset.action==="clear"){clearExpression();return}if(b.dataset.action==="backspace"){const s=input.selectionStart??input.value.length,e=input.selectionEnd??s;if(s===e&&s>0)input.setRangeText("",s-1,e,"end");else input.setRangeText("",s,e,"end");input.focus();return}if(b.dataset.action==="calculate"){calculate();return}if(b.dataset.fn){insert(b.dataset.fn+"(");return}if(b.dataset.insert)insert(b.dataset.insert)});
input.addEventListener("keydown",event=>{if(event.key==="Enter"){event.preventDefault();calculate()}else if(event.key==="Escape"){event.preventDefault();clearExpression()}});
document.addEventListener("keydown",event=>{if(event.ctrlKey||event.metaKey||event.altKey||event.target===input)return;if(event.key==="Escape"){clearExpression();return}if(event.key==="Backspace"){event.preventDefault();const end=input.value.length;input.setRangeText("",Math.max(0,end-1),end,"end");input.focus();return}if(event.key.length===1&&"0123456789.+-*/%^()".includes(event.key))insert(event.key)});
historyBox.addEventListener("click",event=>{const b=event.target.closest("[data-expression]");if(b){input.value=b.dataset.expression;input.focus();calculate()}});
copyButton.addEventListener("click",()=>{if(lastResult)copyText(lastResult)});
document.querySelector("#copy-expression").addEventListener("click",()=>input.value.trim()?copyText(input.value.trim()):say("当前没有表达式可复制。",true));
document.querySelector("#clear-history").addEventListener("click",()=>{items=[];save();renderHistory();say("计算记录已清空。")});
const themeButton=document.querySelector("#theme");try{if(localStorage.getItem("mygpt.calculator.theme")==="dark")document.documentElement.dataset.theme="dark"}catch{}
themeButton.addEventListener("click",()=>{const dark=document.documentElement.dataset.theme!=="dark";if(dark)document.documentElement.dataset.theme="dark";else delete document.documentElement.dataset.theme;try{localStorage.setItem("mygpt.calculator.theme",dark?"dark":"light")}catch{}});
fetch("/healthz").then(r=>r.json()).then(data=>{const version=data.version||"本地服务";document.querySelector("#version").textContent=version;document.querySelector("#footer-version").textContent=version}).catch(()=>{document.querySelector("#footer-version").textContent="本地服务"});
renderHistory();
</script>
</body>
</html>`
