package forum

const forumPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>我的论坛</title>
<style>
:root{font:16px/1.55 system-ui,-apple-system,"Segoe UI",sans-serif;color:#18304a;background:#f5f7fb}
*{box-sizing:border-box}body{margin:0}button,input,textarea,select{font:inherit}a{color:#275bb9;text-decoration:none}a:hover{text-decoration:underline}
header{background:#fff;border-bottom:1px solid #dce5f0}header .wrap{width:min(1080px,100% - 32px);margin:auto;padding:20px 0;display:flex;justify-content:space-between;align-items:center;gap:15px}
h1{margin:0;font-size:26px}h2{margin:0 0 12px;font-size:21px}.muted{color:#63768c}.brand p{margin:2px 0 0}.nav{display:flex;gap:16px;white-space:nowrap}
main{width:min(1080px,100% - 32px);margin:28px auto;display:grid;grid-template-columns:230px minmax(0,1fr);gap:20px}.card{border:1px solid #dce5f0;border-radius:14px;background:white;padding:21px;box-shadow:0 4px 20px #1e345208;margin-bottom:20px}
.topic{display:block;border:0;background:transparent;width:100%;padding:10px;text-align:left;border-radius:9px;cursor:pointer;color:#18304a}.topic:hover,.topic.active{background:#e9f0fc;color:#2457b6}
.post{padding:16px 0;border-bottom:1px solid #e7edf4;cursor:pointer}.post:last-child{border:0}.post strong{display:block;font-size:18px}.post:hover strong{color:#2457b6}
label{display:block;margin:12px 0 5px;font-weight:650}input:not([type=checkbox]),select,textarea{width:100%;padding:10px;border:1px solid #ccd9e8;border-radius:9px;background:white;color:#18304a}textarea{min-height:145px;resize:vertical}button.primary{padding:10px 17px;margin-top:14px;background:#2b62c6;border:0;color:white;border-radius:9px;cursor:pointer}button:disabled{opacity:.6;cursor:wait}.message{min-height:23px;color:#a5343d;white-space:pre-wrap}.body{white-space:pre-wrap;overflow-wrap:anywhere;line-height:1.8}.reply{border-top:1px solid #e5edf5;padding:16px 0}.reply .body{margin-top:8px}.attachment{display:inline-flex;gap:8px;padding:6px 9px;margin:6px 9px 6px 0;background:#edf3fc;border-radius:8px}
.small{font-size:13px}.hidden{display:none!important}.row{display:flex;gap:10px;align-items:center;flex-wrap:wrap}.row>*{flex:1}.row button{flex:0 0 auto}.back{background:none;border:0;color:#275bb9;cursor:pointer;padding:0 0 12px}
.admin-tools{display:flex;gap:8px;margin:10px 0}.admin-button{border:1px solid #c9d7e8;border-radius:8px;background:#f7f9fc;color:#294361;padding:6px 10px;cursor:pointer}.admin-button.danger{color:#a33;border-color:#ead1d1}
.topic-row{display:flex;align-items:center;gap:5px}.topic-row .topic{flex:1;min-width:0}.topic-copy{border:0;background:transparent;color:#57708f;cursor:pointer;padding:6px;font-size:13px}.topic-copy:hover{color:#2457b6}.inline-image{display:block;max-width:min(100%,720px);max-height:70vh;margin:10px 0;border-radius:10px;object-fit:contain}.resource-video,.resource-audio{display:block;max-width:100%;margin:10px 0}.trash-entry{padding:14px 0;border-bottom:1px solid #e7edf4}.trash-entry:last-child{border:0}.post-preview{margin:8px 0;white-space:pre-wrap;overflow-wrap:anywhere;color:#52677f}.search-row{display:flex;gap:8px;margin:12px 0}.search-row input{flex:1}.search-row button{margin:0}
@media(max-width:720px){main{display:block}header .wrap{align-items:flex-start}.nav{flex-wrap:wrap}.card{padding:16px}}
</style></head><body>
<header><div class="wrap"><div class="brand"><h1 id="site-title">我的论坛</h1><p id="site-description" class="muted">分享想法，记录文章。</p></div><nav class="nav"><a href="/">首页</a><a href="/files">文件管理</a><a href="/calculator">计算器</a><a href="#trash" id="trash-link" class="hidden">回收站</a><a href="#settings" id="settings-link">设置</a></nav></div></header>
<main><aside><div class="card"><h2>主题</h2><div id="topics"></div><button class="primary" id="add-topic" type="button">添加主题</button><div id="topic-message" class="message small"></div></div></aside>
<div><section id="feed"><div class="card"><h2>最新帖子</h2><form id="search-form" class="search-row hidden"><input id="search-query" maxlength="200" placeholder="搜索标题、正文或署名"><button class="primary" type="submit">搜索</button><button class="admin-button" id="clear-search" type="button">清除</button></form><div id="posts">加载中…</div><button id="more" class="primary hidden" type="button">加载更多</button></div><div class="card" id="editor"><h2>发布帖子</h2><form id="post-form"><label for="post-topic">主题</label><select id="post-topic" required></select><label for="post-title">标题</label><input id="post-title" maxlength="200" required><label for="post-author">署名</label><input id="post-author" maxlength="80" required value="访客"><label for="post-body">正文（纯文本）</label><textarea id="post-body" maxlength="100000" required></textarea><p id="resource-help" class="muted small hidden">资源渲染开启后可识别 HTTPS 链接、Markdown 链接、Markdown 图片，例如 <code>![说明](https://example.com/image.jpg)</code>。输入的 HTML 不会执行。</p><label for="post-files">附件（单文件最大 20 MB）</label><input id="post-files" type="file" multiple><button class="primary" type="submit">发布</button><div class="message" id="post-message"></div></form></div></section>
<section id="detail" class="card hidden"><button class="back" id="back">← 返回帖子列表</button><h2 id="detail-title"></h2><div id="post-tools"></div><div class="message" id="detail-message"></div><p class="muted small" id="detail-meta"></p><div class="body" id="detail-body"></div><div id="detail-files"></div><h2 style="margin-top:28px">回复</h2><div id="replies"></div><form id="reply-form"><label for="reply-author">署名</label><input id="reply-author" maxlength="80" required value="访客"><label for="reply-body">回复内容</label><textarea id="reply-body" maxlength="20000" required></textarea><label for="reply-files">附件</label><input id="reply-files" type="file" multiple><button type="submit" class="primary">发表回复</button><div class="message" id="reply-message"></div></form></section>
<section id="trash" class="card hidden"><button class="back" id="trash-back">← 返回</button><div class="row"><h2>回收站</h2><button id="empty-trash" class="admin-button danger" type="button">清空回收站</button></div><p class="muted small">删除的帖子、回复和文件会暂存在这里。彻底删除后无法恢复。</p><div id="trash-list">加载中…</div></section>
<section id="settings" class="card hidden"><button class="back" id="settings-back">← 返回</button><h2>论坛设置</h2><p class="muted small">管理员令牌保存在程序数据目录的 config/admin-token 中，仅在本次浏览器会话中使用。</p><form id="settings-form"><label for="admin-token">管理员令牌</label><input id="admin-token" type="password" autocomplete="off"><label for="setting-title">站点名称</label><input id="setting-title" maxlength="100" required><label for="setting-description">站点简介</label><textarea id="setting-description" maxlength="500"></textarea><label><input type="checkbox" id="guest-posts"> 允许访客发布帖子</label><label><input type="checkbox" id="guest-replies"> 允许访客回复</label><label><input type="checkbox" id="resource-rendering"> 启用正文资源渲染（链接、图片和附件预览）</label><label><input type="checkbox" id="search-enabled"> 启用帖子搜索（标题、正文和署名）</label><button type="submit" class="primary">保存设置</button><div id="settings-message" class="message"></div></form></section></div></main>
<script>
const $=q=>document.querySelector(q);let currentTopic="",currentPost="",searchQuery="",offset=0,settings={},attachmentNotice={id:"",text:""};
const err=async r=>{if(!r.ok)throw new Error((await r.text()).trim());return r.status===204?null:r.json()};
const key=()=>$("#admin-token").value.trim()||sessionStorage.getItem("forum-admin-token")||"";
async function api(path,method="GET",body){return err(await fetch(path,{method,headers:{"Content-Type":"application/json",...(key()?{"X-Admin-Token":key()}:{})},body:body?JSON.stringify(body):undefined}))}
function show(id){for(const name of ["feed","detail","trash","settings"])$("#"+name).classList.toggle("hidden",name!==id)}
function date(value){return new Date(value).toLocaleString()}
function make(tag,value,className){const el=document.createElement(tag);el.textContent=value;if(className)el.className=className;return el}
function adminButton(label,handler,danger=false){const button=make("button",label,"admin-button"+(danger?" danger":""));button.type="button";button.onclick=handler;return button}
async function copyText(value,button){try{if(navigator.clipboard&&navigator.clipboard.writeText){await navigator.clipboard.writeText(value)}else{const field=document.createElement("textarea");field.value=value;field.style.position="fixed";field.style.opacity="0";document.body.append(field);field.select();const ok=document.execCommand("copy");field.remove();if(!ok)throw new Error("复制失败")};const old=button.textContent;button.textContent="已复制";setTimeout(()=>button.textContent=old,1300)}catch(error){button.textContent="复制失败";setTimeout(()=>button.textContent="复制",1600)}}
function copyButton(value,label="复制"){return adminButton(label,event=>copyText(value,event.currentTarget))}
function resourceURL(value){if(/^\/api\/files\/[a-f0-9]{32}(?:\/preview)?$/.test(value))return value;try{const url=new URL(value,location.href);return ["http:","https:"].includes(url.protocol)?url.href:""}catch{return ""}}
function imageURL(value){return value.startsWith("/api/files/")||(/^https:\/\//i.test(value)&&/\.(png|jpe?g|gif|webp)(?:[?#].*)?$/i.test(value))}
function appendText(target,value){const lines=value.split("\n");lines.forEach((line,index)=>{if(index)target.append(document.createElement("br"));target.append(document.createTextNode(line))})}
function resourceContent(target,value){target.replaceChildren();if(!settings.enable_resource_rendering){target.textContent=value;return}const pattern=/!\[([^\]]*)\]\((https:\/\/[^\s)]+|\/api\/files\/[a-f0-9]{32}\/preview)\)|\[([^\]]+)\]\((https?:\/\/[^\s)]+|\/api\/files\/[a-f0-9]{32}(?:\/preview)?)\)|(https?:\/\/[^\s<>]+)/g;let last=0,match;while((match=pattern.exec(value))){appendText(target,value.slice(last,match.index));const raw=match[2]||match[4]||match[5],url=resourceURL(raw),alt=match[1],label=match[3];if(!url){appendText(target,match[0]);last=pattern.lastIndex;continue}if((match[1]!==undefined||match[5]&&imageURL(raw))&&(url.startsWith(location.origin)||url.startsWith("/" )||url.startsWith("https://"))){const image=document.createElement("img");image.src=url;image.alt=alt||raw;image.loading="lazy";image.decoding="async";image.referrerPolicy="no-referrer";image.className="inline-image";target.append(image)}else{const link=document.createElement("a");link.href=url;link.textContent=label||raw;link.target="_blank";link.rel="noopener noreferrer";target.append(link)}last=pattern.lastIndex}appendText(target,value.slice(last))}
function attachments(target,files){target.replaceChildren();for(const file of files||[]){if(settings.enable_resource_rendering&&file.content_type&&file.content_type.startsWith("image/")){const image=document.createElement("img");image.src=file.download_url+"/preview";image.alt=file.name;image.loading="lazy";image.className="inline-image";target.append(image)}const span=make("span","","attachment"),preview=make("a","预览"),download=make("a","下载");preview.href=file.download_url+"/preview";preview.target="_blank";preview.rel="noopener noreferrer";download.href=file.download_url;span.append(make("span",file.name),preview,download);target.append(span)}}
async function loadSettings(){settings=await api("/api/forum/settings");$("#site-title").textContent=settings.title;document.title=settings.title;$("#site-description").textContent=settings.description;$("#setting-title").value=settings.title;$("#setting-description").value=settings.description;$("#guest-posts").checked=settings.allow_guest_posts;$("#guest-replies").checked=settings.allow_guest_replies;$("#resource-rendering").checked=!!settings.enable_resource_rendering;$("#search-enabled").checked=!!settings.enable_search;$("#resource-help").classList.toggle("hidden",!settings.enable_resource_rendering);$("#search-form").classList.toggle("hidden",!settings.enable_search);if(!settings.enable_search){searchQuery="";$("#search-query").value=""}$("#trash-link").classList.toggle("hidden",!key());$("#editor").classList.toggle("hidden",!settings.allow_guest_posts&&!key())}
async function loadTopics(){const data=await api("/api/forum/topics");const container=$("#topics"),select=$("#post-topic");container.replaceChildren();select.replaceChildren();const all=make("button","全部主题","topic"+(currentTopic?"":" active"));all.onclick=()=>{currentTopic="";loadTopics();loadPosts(false)};container.append(all);for(const topic of data.items){const row=make("div","","topic-row"),button=make("button",topic.name,"topic"+(topic.id===currentTopic?" active":""));button.title=topic.description;button.onclick=()=>{currentTopic=topic.id;loadTopics();loadPosts(false)};row.append(button,copyButton(topic.name+(topic.description?"\n"+topic.description:""),"复制主题"));container.append(row);const option=document.createElement("option");option.value=topic.id;option.textContent=topic.name;select.append(option)}if(currentTopic)select.value=currentTopic}
async function loadPosts(next){
 if(!next){offset=0;$("#posts").replaceChildren()}
 let path="/api/forum/posts?limit=20&offset="+offset;
 if(currentTopic)path+="&topic_id="+encodeURIComponent(currentTopic);
 if(searchQuery)path+="&q="+encodeURIComponent(searchQuery);
 const data=await api(path);
 for(const post of data.items){
  const row=make("div","","post"),title=make("strong",post.title),meta=make("span",post.author+" · "+date(post.created_at)+" · "+post.reply_count+" 条回复","muted small");
  row.append(title,meta);
  if(searchQuery&&post.body)row.append(make("div",post.body.slice(0,220),"post-preview"));
  const copyValue=post.body?post.title+"\n\n"+post.body:post.title+"\n"+location.origin+"/#post/"+post.id;
  const tools=make("div","","admin-tools"),copy=copyButton(copyValue,post.body?"复制帖子内容":"复制标题和链接");
  copy.onclick=event=>{event.stopPropagation();copyText(copyValue,event.currentTarget)};
  tools.append(copy);row.append(tools);row.onclick=()=>{location.hash="post/"+post.id};$("#posts").append(row);
 }
 offset+=data.items.length;
 $("#more").classList.toggle("hidden",data.items.length<20);
 if(!offset)$("#posts").append(make("p",searchQuery?"没有找到匹配的帖子。":"这里还没有帖子，可以发布第一篇。","muted"));
}
async function showPost(id){
 currentPost=id;show("detail");
 const post=await api("/api/forum/posts/"+encodeURIComponent(id));
 $("#detail-title").textContent=post.title;
 $("#detail-message").textContent=attachmentNotice.id===id?attachmentNotice.text:"";
 $("#detail-meta").textContent=post.author+" · "+date(post.created_at);
 resourceContent($("#detail-body"),post.body);
 attachments($("#detail-files"),post.attachments);
 const tools=$("#post-tools");tools.replaceChildren();
 const actions=make("div","","admin-tools");
 actions.append(copyButton(post.title+"\n\n"+post.body,"复制帖子内容"));
 actions.append(copyButton(location.origin+"/#post/"+id,"复制帖子链接"));
 if(key()){
  actions.append(adminButton("编辑帖子",async()=>{
   const title=prompt("修改标题",post.title);if(title===null)return;
   const body=prompt("修改正文",post.body);if(body===null)return;
   try{await api("/api/forum/posts/"+encodeURIComponent(id),"PUT",{title,body});await showPost(id);await loadPosts(false)}
   catch(error){$("#detail-message").textContent=error.message}
  }));
  actions.append(adminButton("移入回收站",async()=>{
   if(!confirm("将帖子和回复移入回收站？附件会保留并可随帖子恢复。"))return;
   try{await api("/api/forum/posts/"+encodeURIComponent(id),"DELETE");location.hash="";show("feed");await loadPosts(false)}
   catch(error){$("#detail-message").textContent=error.message}
  },true));
 }
 tools.append(actions);
 const container=$("#replies");container.replaceChildren();
 for(const reply of post.replies){
  const row=make("div","","reply"),files=make("div",""),replyBody=make("div","","body");
  row.append(make("div",reply.author+" · "+date(reply.created_at),"muted small"));
  resourceContent(replyBody,reply.body);row.append(replyBody,files);attachments(files,reply.attachments);
  const replyActions=make("div","","admin-tools");
  replyActions.append(copyButton(reply.author+" · "+date(reply.created_at)+"\n"+reply.body,"复制回复"));
  if(key()){
   replyActions.append(adminButton("编辑回复",async()=>{
    const body=prompt("修改回复内容",reply.body);if(body===null)return;
    try{await api("/api/forum/replies/"+encodeURIComponent(reply.id),"PUT",{body});await showPost(id)}
    catch(error){$("#reply-message").textContent=error.message}
   }));
   replyActions.append(adminButton("移入回收站",async()=>{
    if(!confirm("将这条回复移入回收站？附件会保留并可随回复恢复。"))return;
    try{await api("/api/forum/replies/"+encodeURIComponent(reply.id),"DELETE");await showPost(id);await loadPosts(false)}
    catch(error){$("#reply-message").textContent=error.message}
   },true));
  }
  row.append(replyActions);container.append(row);
 }
 if(!post.replies.length)container.append(make("p","还没有回复。","muted"));
 $("#reply-form").classList.toggle("hidden",!settings.allow_guest_replies&&!key());
}
async function upload(input,type,id){const errors=[];for(const file of input.files){const form=new FormData();form.append("file",file);form.append("related_type",type);form.append("related_id",id);try{const res=await fetch("/api/files",{method:"POST",body:form});if(!res.ok)throw new Error((await res.text()).trim())}catch(e){errors.push(file.name+": "+e.message)}}input.value="";return errors}
async function loadTrash(){const container=$("#trash-list");container.replaceChildren(make("p","加载回收站…","muted"));try{const [content,files]=await Promise.all([api("/api/forum/trash"),api("/api/files/trash")]);container.replaceChildren();const addSection=(title,items,render)=>{container.append(make("h3",title));if(!items.length){container.append(make("p","没有项目。","muted small"));return}for(const item of items)container.append(render(item))};addSection("已删除帖子",content.posts||[],item=>{const row=make("div","","trash-entry");row.append(make("strong",item.title),make("div",item.topic_name+" · "+item.author+" · 删除于 "+date(item.deleted_at),"muted small"),make("div",item.body.slice(0,240),"post-preview"));const actions=make("div","","admin-tools");actions.append(adminButton("恢复帖子",async()=>{try{await api("/api/forum/trash/posts/"+encodeURIComponent(item.id)+"/restore","POST");await loadTrash();await loadPosts(false)}catch(error){alert(error.message)}}),adminButton("彻底删除",async()=>{if(!confirm("彻底删除这篇帖子及其回复？附件文件本身会保留。"))return;try{await api("/api/forum/trash/posts/"+encodeURIComponent(item.id),"DELETE");await loadTrash()}catch(error){alert(error.message)}},true));row.append(actions);return row});addSection("已删除回复",content.replies||[],item=>{const row=make("div","","trash-entry");row.append(make("strong","回复于："+item.post_title),make("div",item.author+" · 删除于 "+date(item.deleted_at),"muted small"),make("div",item.body,"post-preview"));const actions=make("div","","admin-tools");actions.append(adminButton("恢复回复",async()=>{try{await api("/api/forum/trash/replies/"+encodeURIComponent(item.id)+"/restore","POST");await loadTrash();await loadPosts(false)}catch(error){alert(error.message)}}),adminButton("彻底删除",async()=>{if(!confirm("彻底删除这条回复？附件文件本身会保留。"))return;try{await api("/api/forum/trash/replies/"+encodeURIComponent(item.id),"DELETE");await loadTrash()}catch(error){alert(error.message)}},true));row.append(actions);return row});addSection("已删除文件",files.items||[],item=>{const row=make("div","","trash-entry");row.append(make("strong",item.name),make("div",item.size+" 字节 · 删除于 "+date(item.deleted_at),"muted small"));const actions=make("div","","admin-tools");actions.append(adminButton("恢复文件",async()=>{try{await api("/api/files/"+encodeURIComponent(item.id)+"/restore","POST");await loadTrash()}catch(error){alert(error.message)}}),adminButton("彻底删除",async()=>{if(!confirm("彻底删除文件 "+item.name+"？此操作无法恢复。"))return;try{await api("/api/files/"+encodeURIComponent(item.id)+"/purge","DELETE");await loadTrash()}catch(error){alert(error.message)}},true));row.append(actions);return row})}catch(error){container.replaceChildren(make("p",error.message,"message"))}}
$("#post-form").onsubmit=async e=>{e.preventDefault();const button=e.submitter,notice=$("#post-message");button.disabled=true;notice.textContent="发布中…";try{const post=await api("/api/forum/posts","POST",{topic_id:$("#post-topic").value,title:$("#post-title").value,body:$("#post-body").value,author:$("#post-author").value});const errors=await upload($("#post-files"),"post",post.id);$("#post-form").reset();attachmentNotice={id:post.id,text:errors.length?"帖子已发布，但这些附件失败：\n"+errors.join("\n"):""};notice.textContent="";location.hash="post/"+post.id;await showPost(post.id);await loadPosts(false)}catch(error){notice.textContent=error.message}finally{button.disabled=false}};
$("#reply-form").onsubmit=async e=>{e.preventDefault();const button=e.submitter,notice=$("#reply-message");button.disabled=true;notice.textContent="发表中…";try{const reply=await api("/api/forum/posts/"+encodeURIComponent(currentPost)+"/replies","POST",{author:$("#reply-author").value,body:$("#reply-body").value});const errors=await upload($("#reply-files"),"reply",reply.id);$("#reply-body").value="";await showPost(currentPost);notice.textContent=errors.length?"回复已发表，但附件失败：\n"+errors.join("\n"):""}catch(error){notice.textContent=error.message}finally{button.disabled=false}};
$("#settings-form").onsubmit=async e=>{e.preventDefault();const message=$("#settings-message");try{sessionStorage.setItem("forum-admin-token",$("#admin-token").value.trim());await api("/api/forum/settings","PUT",{title:$("#setting-title").value,description:$("#setting-description").value,allow_guest_posts:$("#guest-posts").checked,allow_guest_replies:$("#guest-replies").checked,enable_resource_rendering:$("#resource-rendering").checked,enable_search:$("#search-enabled").checked});await loadSettings();message.textContent="已保存"}catch(error){message.textContent=error.message}};
$("#add-topic").onclick=async()=>{const name=prompt("主题名称（需管理员令牌）");if(!name)return;const description=prompt("主题简介")||"";let token=key();if(!token){token=prompt("请输入数据目录 config/admin-token 中的管理员令牌")||"";sessionStorage.setItem("forum-admin-token",token)}try{await api("/api/forum/topics","POST",{name,description});$("#topic-message").textContent="主题已创建";await loadTopics()}catch(error){$("#topic-message").textContent=error.message}};
$("#more").onclick=()=>loadPosts(true).catch(e=>$("#posts").append(make("p",e.message,"message")));
$("#search-form").onsubmit=event=>{event.preventDefault();searchQuery=$("#search-query").value.trim();loadPosts(false).catch(error=>$("#posts").replaceChildren(make("p",error.message,"message")))};
$("#clear-search").onclick=()=>{$("#search-query").value="";searchQuery="";loadPosts(false).catch(error=>$("#posts").replaceChildren(make("p",error.message,"message")))};
$("#empty-trash").onclick=async()=>{if(!confirm("永久清空回收站中的帖子、回复和文件？此操作无法撤销。"))return;try{await api("/api/forum/trash","DELETE");await api("/api/files/trash","DELETE");await loadTrash()}catch(error){alert(error.message)}};
$("#back").onclick=()=>{location.hash=""};$("#settings-back").onclick=()=>{location.hash=""};$("#trash-back").onclick=()=>{location.hash=""};
async function route(){try{if(location.hash==="#settings"){show("settings");return}if(location.hash==="#trash"){show("trash");await loadTrash();return}if(location.hash.startsWith("#post/")){await showPost(location.hash.slice(6));return}show("feed")}catch(error){$("#detail-body").textContent=error.message}}
window.addEventListener("hashchange",route);
(async()=>{try{await loadSettings();await loadTopics();await loadPosts(false);await route()}catch(error){$("#posts").textContent=error.message}})();
</script></body></html>`
