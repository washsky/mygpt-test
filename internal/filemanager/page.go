const filePage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>文件管理 · mygpt-test</title>
<style>
:root{color-scheme:light;--ink:#17243b;--muted:#65738a;--accent:#2865e8;--line:#e3e9f2}
*{box-sizing:border-box}body{margin:0;background:#f3f6fb;color:var(--ink);font:15px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif}
main{width:min(900px,100% - 32px);margin:48px auto}.top{display:flex;align-items:center;justify-content:space-between;gap:16px}
h1{margin:0;font-size:30px}.sub{color:var(--muted);margin:6px 0 22px}a{color:var(--accent);text-decoration:none}
.card{padding:22px;margin:16px 0;background:white;border:1px solid var(--line);border-radius:16px;box-shadow:0 8px 30px #19365c08}
form{display:grid;grid-template-columns:1fr 1fr 1fr auto;gap:10px;align-items:end}
label{display:grid;gap:6px;color:var(--muted);font-size:13px}input,select,button{height:42px;padding:0 12px;border:1px solid var(--line);border-radius:10px;background:#fff;font:inherit;color:var(--ink)}
button{background:var(--accent);color:white;border:0;font-weight:650;cursor:pointer}button.danger{height:34px;padding:0 10px;background:#fff0f0;color:#b12b36}
.notice{min-height:24px;color:var(--muted);margin-top:12px}.list{display:grid;gap:8px}.item{display:grid;grid-template-columns:minmax(0,1fr) auto auto;gap:12px;align-items:center;padding:12px;border:1px solid var(--line);border-radius:12px}
.name{font-weight:650;overflow-wrap:anywhere}.meta{font-size:12px;color:var(--muted)}.empty{color:var(--muted);padding:16px 0}
@media(max-width:720px){form{grid-template-columns:1fr 1fr}form button{align-self:end}.item{grid-template-columns:1fr auto}.item .meta{grid-column:1}.item a,.item button{grid-row:1/3}}
</style></head>
<body><main>
<div class="top"><h1>文件管理</h1><a href="/">返回计算器</a></div>
<p class="sub">上传文件会保存在程序旁的数据目录中。单文件上限 20 MB；目前支持关联帖子、回复或用户。</p>
<section class="card">
<form id="upload">
<label>选择文件<input type="file" name="file" required></label>
<label>关联类型<select name="related_type"><option value="">暂不关联</option><option value="post">帖子</option><option value="reply">回复</option><option value="user">用户</option></select></label>
<label>关联记录 ID<input name="related_id" maxlength="128" placeholder="可先留空"></label>
<button type="submit">上传文件</button>
</form><div class="notice" id="notice" role="status"></div>
</section>
<section class="card"><h2>已上传文件</h2><div class="list" id="list"><div class="empty">加载中…</div></div></section>
</main>
<script>
const list=document.querySelector("#list"),notice=document.querySelector("#notice");
const sizeText=n=>n<1024? n+" B":n<1048576?(n/1024).toFixed(1)+" KB":(n/1048576).toFixed(1)+" MB";
async function refresh(){try{const res=await fetch("/api/files");if(!res.ok)throw new Error("加载文件列表失败");const data=await res.json();if(!data.items.length){list.innerHTML='<div class="empty">还没有文件，选择文件后上传。</div>';return}list.replaceChildren(...data.items.map(file=>{const row=document.createElement("div");row.className="item";const info=document.createElement("div");const name=document.createElement("div");name.className="name";name.textContent=file.name;const meta=document.createElement("div");meta.className="meta";meta.textContent=sizeText(file.size)+" · "+new Date(file.uploaded_at).toLocaleString()+(file.association?.type?" · "+file.association.type+": "+file.association.id:"");info.append(name,meta);const download=document.createElement("a");download.href=file.download_url;download.textContent="下载";const remove=document.createElement("button");remove.className="danger";remove.textContent="删除";remove.onclick=async()=>{if(!confirm("删除 "+file.name+"？"))return;const response=await fetch(file.download_url,{method:"DELETE"});notice.textContent=response.ok?"已删除":"删除失败";refresh()};row.append(info,download,remove);return row}))}catch(error){list.innerHTML='<div class="empty"></div>';list.firstChild.textContent=error.message}}
document.querySelector("#upload").addEventListener("submit",async event=>{event.preventDefault();notice.textContent="正在上传…";try{const response=await fetch("/api/files",{method:"POST",body:new FormData(event.currentTarget)});if(!response.ok)throw new Error(await response.text());event.currentTarget.reset();notice.textContent="上传成功";await refresh()}catch(error){notice.textContent=error.message}});
refresh();
</script></body></html>`;
