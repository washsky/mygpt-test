// Run: npm install --no-save playwright && node tests/browser.mjs
// Uses the actual embedded HTML/JS with a deterministic API fixture; no Go server required.
import { createRequire } from 'node:module';
import assert from 'node:assert/strict';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || (process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES ? process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES + '/playwright' : 'playwright'));
import { html,fileHTML,calculatorHTML,accountHTML } from './embedded.mjs';
const browser=await chromium.launch({headless:true,executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH,args:['--no-sandbox']});
const context=await browser.newContext();
const page=await context.newPage();
const errors=[];page.on('pageerror',error=>errors.push(error.message));
const settings={title:'山间札记',description:'记录技术、生活与每一次新的发现。',allow_guest_posts:true,allow_guest_replies:true,enable_search:true};
const topics=[{id:'a',name:'日常记录',description:'生活'},{id:'b',name:'技术笔记',description:'技术'}];
const post=id=>({id,title:'给未来的自己留一份记录 · '+id,author:'访客',topic_id:'a',created_at:'2026-01-01T12:00:00Z',reply_count:0,body:'这一篇笔记记录生活与技术。\n'+('文字与长链接 https://example.com/ '+ 'a'.repeat(120)+'\n').repeat(6),replies:[],attachments:[]});
const imageID='a'.repeat(32),replyID='c'.repeat(32);
const appearancePost={...post('appearance'),author:'long_account_name_123456789012345',user_id:'author-id',user_uid:12345,client_info:{browser:'Chromium',os:'Linux'},body:'正文的第一行。\n> 一段引用文字。\n引用回复：/#reply/'+replyID+'\n![图片](/api/files/'+imageID+'/preview)\n[附件下载](/api/files/'+imageID+')\n'+('long_text_'.repeat(40)),attachments:[{id:imageID,name:'inline.png',content_type:'image/png',download_url:'/api/files/'+imageID}],replies:[{id:replyID,author:'回复者',created_at:'2026-01-01T13:00:00Z',body:'引用回复功能与正文边框独立。',attachments:[]}]};
const markdownBody='## Markdown 标题\n\n**粗体**与 *斜体* ~~删除线~~\n\n| 列一 | 列二 |\n|---|---|\n| 长表格内容 | '+('cell'.repeat(50))+' |\n\n~~~go\n'+('code_'.repeat(60))+'\n~~~\n\n> 引用\n\n- [x] 完成\n\n![图片](/api/files/'+imageID+'/preview)';
const markdownHTML='<h2 id="markdown-title">Markdown 标题</h2><p><strong>粗体</strong>与 <em>斜体</em> <del>删除线</del></p><table><thead><tr><th>列一</th><th>列二</th></tr></thead><tbody><tr><td>长表格内容</td><td>'+('cell'.repeat(50))+'</td></tr></tbody></table><pre><code class="language-go">'+('code_'.repeat(60))+'</code></pre><blockquote><p>引用</p></blockquote><ul><li><input type="checkbox" disabled checked>完成</li></ul><p><img alt="图片" src="/api/files/'+imageID+'/preview"></p><p><a href="#markdown-title">返回标题</a></p>';
const markdownPost={...post('markdown'),markdown:true,body:markdownBody,body_html:markdownHTML,replies:[{id:replyID,author:'回复者',created_at:'2026-01-01T13:00:00Z',body:markdownBody,markdown:true,body_html:markdownHTML,attachments:[]}]};
appearancePost.replies[0].attachments=appearancePost.attachments;
let listCalls=0,failList=false,slow=false,failDetail=false,settingsDelay=0,failSettings=false,fixtureUser=null,authDelay=0;
await page.route('http://blog.test/**',async route=>{
 const url=new URL(route.request().url());let body={};
 if(url.pathname==='/')return route.fulfill({contentType:'text/html',body:html});
 if(url.pathname==='/files')return route.fulfill({contentType:'text/html',body:fileHTML});
 if(url.pathname==='/calculator')return route.fulfill({contentType:'text/html',body:calculatorHTML});
 if(url.pathname==='/account')return route.fulfill({contentType:'text/html',body:accountHTML});
 if(url.pathname==='/api/files/'+imageID+'/preview')return route.fulfill({contentType:'image/png',body:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aG14AAAAASUVORK5CYII=','base64')});
 if(url.pathname==='/api/auth/me'){if(authDelay)await new Promise(resolve=>setTimeout(resolve,authDelay));body={user:fixtureUser,csrf_token:'',allow_registration:true,first_registration:false};}
 else if(url.pathname==='/api/forum/settings'){if(settingsDelay)await new Promise(resolve=>setTimeout(resolve,settingsDelay));if(failSettings)return route.fulfill({status:503,body:'unavailable'});body=settings;}
 else if(url.pathname==='/api/forum/topics')body={items:topics};
 else if(url.pathname==='/api/forum/posts'){
  listCalls++;
  if(failList){failList=false;return route.fulfill({status:503,body:'unavailable'})}
  if(slow)await new Promise(resolve=>setTimeout(resolve,200));
  body={items:url.searchParams.has('q')?[]:Array.from({length:20},(_,i)=>post(String(Number(url.searchParams.get('offset')||0)+i)))};
 }else if(url.pathname.startsWith('/api/forum/posts/')){
  const id=url.pathname.split('/').pop();
  if(id==='slow')await new Promise(resolve=>setTimeout(resolve,300));
  if(failDetail)return route.fulfill({status:404,body:'not found'});
  body=id==='markdown'?markdownPost:id==='appearance'?appearancePost:post(id);
 }else body={items:[]};
 return route.fulfill({contentType:'application/json',body:JSON.stringify(body)});
});
try{
 await page.goto('http://blog.test/');await page.waitForFunction(()=>document.querySelectorAll('.post').length===20);
 assert.equal(await page.locator('#posts').getAttribute('aria-busy'),'false');
 assert.equal(await page.locator('.post-avatar').first().isVisible(),false,'list avatars should default off');
 assert.equal(await page.locator('#content-heading').count(),0,'the article should not add a visible body heading');
 // Reading starts with posts; the composer only opens on request.
 assert.equal(await page.locator('#editor').isVisible(),false);
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:900});
  const first=await page.locator('.post-title').first().boundingBox();
  if(process.env.FORUM_SCREENSHOT_PREFIX)await page.screenshot({path:process.env.FORUM_SCREENSHOT_PREFIX+'-'+width+'.png'});
  assert.ok(first&&first.y<450,'first post is pushed below the first screen at '+width+': '+JSON.stringify(first));
  assert.ok(await page.evaluate(()=>parseFloat(getComputedStyle(document.querySelector('.post-title')).fontSize)>parseFloat(getComputedStyle(document.querySelector('#site-title')).fontSize)));
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'feed overflow at '+width);
 }
 await page.setViewportSize({width:1440,height:1000});
 // Navigation from another page enters the forum first; a second click opens topics.
 assert.equal(await page.locator('aside').count(),0);assert.equal(await page.locator('#topic-panel').isVisible(),false);
 for(const path of ['/files','/calculator']){
  await page.goto('http://blog.test'+path);await page.locator('#workspace-forum').click();await page.waitForFunction(()=>document.querySelectorAll('.post').length===20);
  assert.equal(await page.locator('#topic-panel').isVisible(),false);
  await page.locator('#workspace-forum').click();await page.locator('#topic-panel').waitFor();
  await page.getByRole('button',{name:'技术笔记',exact:true}).click();await page.waitForFunction(()=>document.querySelector('#feed-title').textContent==='技术笔记');assert.equal(await page.locator('#topic-panel').isVisible(),false);
 }
 await page.goto('http://blog.test/#settings');await page.locator('#settings').waitFor();await page.locator('#workspace-forum').click();await page.locator('#feed').waitFor();assert.equal(await page.locator('#topic-panel').isVisible(),false);
 await page.locator('#workspace-forum').click();await page.locator('#topic-panel').waitFor();await page.locator('#feed-title').click();assert.equal(await page.locator('#topic-panel').isVisible(),false);
 await page.locator('#workspace-forum').focus();await page.keyboard.press('ArrowDown');await page.locator('#topic-panel').waitFor();await page.keyboard.press('Escape');assert.equal(await page.locator('#workspace-forum').getAttribute('aria-expanded'),'false');assert.equal(await page.locator('#workspace-forum').evaluate(el=>el===document.activeElement),true);
 for(const width of [360,390,768,1440]){await page.setViewportSize({width,height:900});await page.locator('#workspace-forum').click();const panel=await page.locator('#topic-panel').boundingBox();assert.ok(panel&&panel.x>=0&&panel.x+panel.width<=width+1,`topic panel overflow at ${width}`);await page.keyboard.press('Escape')}
 await page.setViewportSize({width:1440,height:1000});
 // Parallel pagination clicks must request only one page.
 slow=true;const before=listCalls;
 await page.evaluate(()=>{loadPosts(true);loadPosts(true)});
 await page.waitForFunction(()=>document.querySelectorAll('.post').length===40);
 assert.equal(listCalls-before,1);slow=false;
 // Failure keeps already loaded items and exposes a working retry.
 failList=true;await page.locator('#more').click();await page.getByRole('button',{name:'重试',exact:true}).waitFor();
 assert.equal(await page.locator('.post').count(),40);
 await page.getByRole('button',{name:'重试',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.post').length===60);
 // Search empty state and clear.
 await page.locator('#search-query').fill('不存在');await page.locator('#search-form button[type=submit]').click();
 await page.getByText('没有找到相关笔记',{exact:true}).waitFor();await page.locator('#clear-search').click();await page.waitForFunction(()=>document.querySelectorAll('.post').length===20);
 // Topic refresh preserves the in-progress draft selection.
 await page.locator('#compose-link').click();await page.locator('#editor').waitFor();assert.equal(await page.locator('#feed').isVisible(),false);
 assert.equal(await page.locator('#post-title').evaluate(el=>el===document.activeElement),true);
 await page.locator('#post-topic').selectOption('b');await page.evaluate(()=>loadTopics());assert.equal(await page.locator('#post-topic').inputValue(),'b');
 await page.locator('#post-body').fill('尚未发布的草稿');await page.waitForTimeout(350);await page.reload();await page.waitForFunction(()=>document.querySelector('#post-body').value==='尚未发布的草稿');
 assert.equal(await page.locator('#post-topic').inputValue(),'b');
 await page.locator('#compose-back').click();await page.locator('#feed').waitFor();assert.equal(await page.locator('#editor').isVisible(),false);
 await page.locator('#compose-link').click();await page.locator('#editor').waitFor();assert.equal(await page.locator('#post-body').inputValue(),'尚未发布的草稿');
 // A slow old detail request cannot overwrite the next selected post.
 await page.evaluate(()=>{location.hash='post/slow'});await page.waitForTimeout(50);await page.evaluate(()=>{location.hash='post/1'});
 await page.waitForFunction(()=>document.title.startsWith('给未来的自己留一份记录 · 1'));await page.waitForTimeout(350);
 assert.match(await page.locator('#detail-title').textContent(),/ · 1$/);
 assert.match(await page.locator('#reading-time').textContent(),/分钟阅读/);
 // Dark theme survives reload and small screens do not overflow.
 await page.evaluate(()=>document.querySelector('#workspace-theme').click());await page.reload();
 assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');
 await page.waitForFunction(()=>document.querySelector('#detail-title').textContent.endsWith(' · 1'));
 for(const width of [360,390,768,1440]){await page.setViewportSize({width,height:900});assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow at ${width}`)}
 await page.setViewportSize({width:1440,height:1000});
 if(process.env.SCREENSHOT_PATH)await page.screenshot({path:process.env.SCREENSHOT_PATH,fullPage:true});
 // Appearance controls change layout without rebuilding content or the list.
 const originalSettings={...settings};
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://blog.test/');await page.locator('#posts[aria-busy=false]').waitFor();
  await page.evaluate(()=>window.firstRow=document.querySelector('.post'));
  for(const avatars of [false,true])for(const bubble of [false,true]){
   settings.enable_list_avatars=avatars;settings.enable_body_bubble=bubble;await page.evaluate(()=>loadSettings());
   assert.equal(await page.locator('.post-avatar').first().isVisible(),avatars);
   if(avatars){
    const avatar=await page.locator('.post-avatar').first().boundingBox(),main=await page.locator('.post-main').first().boundingBox();
    assert.ok(Math.abs(avatar.y+avatar.height/2-main.y-main.height/2)<0.5,'avatar is not centered with the entire post row at '+width);
    if(process.env.FORUM_SCREENSHOT_PREFIX&&bubble)await page.screenshot({path:process.env.FORUM_SCREENSHOT_PREFIX+'-avatar-'+width+'.png'});
   }
   assert.equal(await page.evaluate(()=>window.firstRow===document.querySelector('.post')),true);
   const first=await page.locator('.post').first().boundingBox();if(width>=390)assert.ok(first.height<110,'post rows are too sparse at '+width+': '+first.height);
   assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'appearance feed overflow at '+width);
  }
 }
 // Every combination of line tools, quotes and resource rendering works with either body style.
 await page.setViewportSize({width:320,height:900});await page.goto('http://blog.test/#post/appearance');await page.locator('#detail-title').filter({hasText:'appearance'}).waitFor();
 for(const bubble of [false,true])for(const replyBubble of [false,true])for(let mask=0;mask<16;mask++){
  Object.assign(settings,{enable_body_bubble:bubble,enable_reply_bubble:replyBubble,enable_line_numbers:!!(mask&1),enable_line_copy:!!(mask&2),enable_quote_replies:!!(mask&4),enable_resource_rendering:!!(mask&8),enable_client_info:true,client_browser:true,client_os:true});
  await page.evaluate(async()=>{await loadSettings();await showPost('appearance')});
  assert.equal(await page.locator('#detail-body .line-number').count(),mask&1?appearancePost.body.split('\n').length:0);
  assert.equal(await page.locator('#detail-body .line-copy').count(),mask&2?appearancePost.body.split('\n').length:0);
  if(mask&3)assert.ok((await page.locator('#detail-body .line-text').first().boundingBox()).width>=130,'line tools squeezed the body: '+bubble+'/'+mask);
  assert.equal(await page.locator('#detail-body a[href="/#reply/'+replyID+'"]').count(),mask&4?1:0);
  assert.equal(await page.locator('#detail-body img').count(),mask&8?1:0);
  assert.equal(await page.locator('#article-content #detail-files img').count(),1);
  assert.equal(await page.locator('#replies button').filter({hasText:'引用回复'}).count(),mask&4?1:0);
  assert.equal(await page.locator('#article-content').evaluate(el=>parseFloat(getComputedStyle(el).borderRadius)>0),bubble);
  assert.equal(await page.locator('.reply-content').first().evaluate(el=>parseFloat(getComputedStyle(el).borderRadius)>0),replyBubble);
  assert.equal(await page.locator('.reply-content .line-number').count(),mask&1?1:0);
  assert.equal(await page.locator('.reply-content .line-copy').count(),mask&2?1:0);
  assert.equal(await page.locator('.reply-content .reply-files img').count(),1);
  if(mask&3)assert.ok((await page.locator('.reply-content .line-text').first().boundingBox()).width>=130,'reply tools squeezed the content: '+replyBubble+'/'+mask);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'body feature combination overflow: '+bubble+'/'+mask);
 }
 await page.evaluate(()=>{window.bodyBefore=document.getElementById('detail-body').firstChild;window.replyBefore=document.querySelector('.reply-content .body').firstChild});
 settings.enable_body_bubble=false;settings.enable_reply_bubble=false;settings.enable_list_avatars=false;await page.evaluate(()=>loadSettings());
 assert.equal(await page.evaluate(()=>window.bodyBefore===document.getElementById('detail-body').firstChild&&window.replyBefore===document.querySelector('.reply-content .body').firstChild),true);
 if(process.env.FORUM_SCREENSHOT_PREFIX){
  for(const theme of ['light','dark'])for(const bubble of [false,true]){
   settings.enable_body_bubble=bubble;settings.enable_reply_bubble=bubble;await page.evaluate(async theme=>{document.documentElement.dataset.theme=theme;await loadSettings()},theme);
   await page.locator('#detail-title').scrollIntoViewIfNeeded();await page.screenshot({path:process.env.FORUM_SCREENSHOT_PREFIX+'-detail-'+theme+'-'+bubble+'.png'});
   await page.locator('.reply').first().scrollIntoViewIfNeeded();await page.screenshot({path:process.env.FORUM_SCREENSHOT_PREFIX+'-reply-'+theme+'-'+bubble+'.png'});
  }
 }
 // Markdown is rendered as one document; source-line controls cannot split its blocks.
 for(const [key,value] of Object.entries({enable_line_numbers:false,enable_line_copy:false,default_post_markdown:false,default_reply_markdown:false}))settings[key]=value;
 await page.goto('http://blog.test/#post/markdown');await page.locator('#detail-body .markdown-rendered h2').waitFor();
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:900});
  for(let mask=0;mask<16;mask++){
   Object.assign(settings,{enable_line_numbers:!!(mask&1),enable_line_copy:!!(mask&2),enable_resource_rendering:!!(mask&4),enable_body_bubble:!!(mask&8),enable_reply_bubble:!(mask&8)});
   await page.evaluate(async()=>{await loadSettings();await showPost('markdown')});
   assert.equal(await page.locator('#detail-body .markdown-rendered table').count(),1);assert.equal(await page.locator('.reply .markdown-rendered table').count(),1);
   assert.equal(await page.locator('#detail-body .markdown-rendered pre code').textContent(),'code_'.repeat(60));
   assert.equal(await page.locator('#detail-body .markdown-rendered input').isDisabled(),true);
   assert.equal(await page.locator('#detail-body .markdown-source-tools').count(),mask&3?1:0);
   assert.equal(await page.locator('#detail-body .markdown-source-lines .line-number').count(),mask&1?markdownBody.split('\n').length:0);
   assert.equal(await page.locator('#detail-body .markdown-source-lines .line-copy').count(),mask&2?markdownBody.split('\n').length:0);
   assert.equal(await page.locator('#detail-body .markdown-source-lines a').count(),0);
   assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'Markdown overflow at '+width+'/'+mask);
  }
 }
 await page.setViewportSize({width:390,height:900});
 const markdownHash=await page.evaluate(()=>location.hash);
 await page.locator('#detail-body .markdown-rendered').getByRole('link',{name:'返回标题',exact:true}).click();assert.equal(await page.evaluate(()=>location.hash),markdownHash);
 const allMarkdownIDs=await page.locator('.markdown-rendered [id]').evaluateAll(els=>els.map(el=>el.id));assert.equal(new Set(allMarkdownIDs).size,allMarkdownIDs.length);
 await page.locator('#detail-body .markdown-source-tools>summary').click();
 await page.evaluate(()=>{window.workspaceShareText=async value=>{window.copiedMarkdownLine=value;return false}});
 await page.locator('#detail-body .markdown-source-lines .line-copy').first().click();await page.waitForFunction(()=>window.copiedMarkdownLine==='## Markdown 标题');
 assert.deepEqual(await page.locator('#detail-body .markdown-source-lines .line-text').allTextContents(),markdownBody.split('\n'));
 if(process.env.FORUM_SCREENSHOT_PREFIX)await page.screenshot({path:process.env.FORUM_SCREENSHOT_PREFIX+'-markdown.png'});
 // A restored draft keeps its explicit choice even if site defaults change.
 settings.default_post_markdown=true;settings.default_reply_markdown=true;
 await page.evaluate(()=>localStorage.removeItem('workspace-post-draft'));await page.goto('http://blog.test/#compose');await page.reload();await page.locator('#post-title').waitFor();
 assert.equal(await page.locator('#post-markdown').isChecked(),true);
 await page.locator('#post-markdown').uncheck();await page.locator('#post-body').fill('## 保留纯文本草稿');await page.waitForTimeout(350);await page.reload();await page.locator('#post-title').waitFor();
 assert.equal(await page.locator('#post-markdown').isChecked(),false);
 await page.evaluate(()=>loadSettings());assert.equal(await page.locator('#post-markdown').isChecked(),false);
 await page.goto('http://blog.test/#post/markdown');await page.locator('#reply-markdown').waitFor();assert.equal(await page.locator('#reply-markdown').isChecked(),true);
 await page.locator('#reply-markdown').uncheck();await page.evaluate(()=>loadSettings());assert.equal(await page.locator('#reply-markdown').isChecked(),false);
 for(const key of Object.keys(settings))delete settings[key];Object.assign(settings,originalSettings);
 await page.setViewportSize({width:1440,height:1000});await page.goto('http://blog.test/#post/1');await page.locator('#detail-title').filter({hasText:' · 1'}).waitFor();
 // Missing articles clear old metadata rather than presenting stale content.
 failDetail=true;await page.evaluate(()=>{location.hash='post/missing'});await page.getByText('暂时无法打开内容',{exact:true}).waitFor();
 assert.equal(await page.locator('#detail-meta').textContent(),'');
 // The clipboard entry must not disappear while each new page awaits settings.
 settings.enable_clipboard=true;
 await page.goto('http://blog.test/');await page.locator('#workspace-clipboard').waitFor();
 for(const path of ['/files','/calculator','/account']){
  settingsDelay=600;await page.goto('http://blog.test'+path);
  assert.equal(await page.locator('#workspace-clipboard').isVisible(),true,'clipboard disappeared before settings on '+path);
  await page.waitForTimeout(100);assert.equal(await page.locator('#workspace-clipboard').isVisible(),true);
  await page.waitForFunction(()=>window.workspaceOptions.title==='山间札记');
 }
 settingsDelay=0;await page.locator('#workspace-forum').click();await page.locator('#workspace-clipboard').waitFor();
 await page.evaluate(()=>window.clipboardEntry=document.getElementById('workspace-clipboard'));
 await page.locator('#workspace-forum').click();await page.keyboard.press('Escape');
 await page.locator('.workspace-nav>details>summary').click();await page.keyboard.press('Escape');
 assert.equal(await page.evaluate(()=>window.clipboardEntry===document.getElementById('workspace-clipboard')),true);
 await page.locator('#workspace-clipboard').click();await page.locator('#clipboard-dialog').waitFor();await page.locator('#clipboard-close').click();
 settings.enable_clipboard=false;await page.evaluate(()=>window.dispatchEvent(new Event('workspace-settings-saved')));
 await page.waitForFunction(()=>document.documentElement.dataset.clipboardEnabled==='false');
 assert.equal(await page.locator('#workspace-clipboard').isVisible(),false);
 settingsDelay=600;await page.goto('http://blog.test/files');assert.equal(await page.locator('#workspace-clipboard').isVisible(),false);
 await page.waitForFunction(()=>window.workspaceOptions.title==='山间札记');
 settingsDelay=0;settings.enable_clipboard=true;await page.evaluate(()=>window.dispatchEvent(new Event('workspace-settings-saved')));await page.locator('#workspace-clipboard').waitFor();
 failSettings=true;await page.goto('http://blog.test/calculator');assert.equal(await page.locator('#workspace-clipboard').isVisible(),true);
 await page.waitForFunction(()=>document.querySelector('#connection-status').textContent.includes('连接失败'));
 assert.equal(await page.locator('#workspace-clipboard').isVisible(),true);
 // Mobile controls keep their own cells as labels, selection and pages change.
 failSettings=false;settingsDelay=0;settings.enable_clipboard=true;
 fixtureUser={id:'layout-user',username:'long_account_name_123456789012345',uid:12345,role:'member'};authDelay=600;
 const menuCells=()=>page.locator('.workspace-nav').evaluate(nav=>Array.from(nav.children,el=>{const r=el.getBoundingClientRect();return [r.x,r.y,r.width,r.height].map(n=>Math.round(n*10)/10)}));
 for(const width of [320,360,390,430,600,760]){
  await page.setViewportSize({width,height:900});await page.goto('http://blog.test/');
  const initial=await menuCells();assert.equal(initial.length,6);
  await page.waitForFunction(()=>document.querySelector('#workspace-account').textContent.includes('UID 12345'));
  assert.deepEqual(await menuCells(),initial,'account label moved mobile controls at '+width);
  assert.ok(initial.slice(0,3).every(r=>r[1]===initial[0][1])&&initial.slice(3).every(r=>r[1]===initial[3][1])&&initial[3][1]>initial[0][1],'mobile rows are not three plus three');
  await page.locator('.workspace-nav>details>summary').click();assert.deepEqual(await menuCells(),initial);
  await page.locator('a[data-section=settings]').click();await page.locator('#settings').waitFor();assert.deepEqual(await menuCells(),initial);
  await page.locator('#workspace-forum').click();await page.locator('#feed').waitFor();assert.deepEqual(await menuCells(),initial);
  await page.locator('#workspace-forum').click();await page.locator('#topic-panel').waitFor();assert.deepEqual(await menuCells(),initial);await page.keyboard.press('Escape');
  await page.locator('.workspace-nav a[href="/files"]').click();assert.deepEqual(await menuCells(),initial,'file page moved mobile controls at '+width);
  await page.waitForFunction(()=>document.querySelector('#workspace-account').textContent.includes('UID 12345'));assert.deepEqual(await menuCells(),initial);
  await page.locator('.workspace-nav a[href="/calculator"]').click();assert.deepEqual(await menuCells(),initial,'calculator page moved mobile controls at '+width);
  await page.waitForFunction(()=>document.querySelector('#workspace-account').textContent.includes('UID 12345'));
  await page.evaluate(()=>document.getElementById('workspace-clipboard').textContent='剪贴板 · 有更新');assert.deepEqual(await menuCells(),initial);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'mobile menu overflow at '+width);
 }
 settings.enable_clipboard=false;await page.evaluate(()=>window.dispatchEvent(new Event('workspace-settings-saved')));
 const beforeDisable=await menuCells();await page.waitForFunction(()=>document.documentElement.dataset.clipboardEnabled==='false');assert.deepEqual((await menuCells()).slice(0,5),beforeDisable.slice(0,5));
 // Direct compose links respect guest publishing settings and keep a login route.
 fixtureUser=null;authDelay=0;settings.allow_guest_posts=false;
 await page.goto('http://blog.test/#compose');await page.locator('#compose-permission').waitFor();
 assert.equal(await page.locator('#post-form').isVisible(),false);assert.equal(await page.locator('#feed').isVisible(),false);
 assert.equal(await page.locator('#compose-permission a').getAttribute('href'),'/account');
 fixtureUser={id:'writer',username:'已登录作者',uid:2,role:'member'};
 await page.reload();await page.locator('#post-title').waitFor();assert.equal(await page.locator('#compose-permission').isVisible(),false);
 await page.locator('#compose-back').click();await page.locator('#feed').waitFor();
 await page.locator('.post-link').first().click();await page.getByText('暂时无法打开内容',{exact:true}).waitFor();assert.equal(await page.locator('#feed').isVisible(),false);
 assert.deepEqual(errors,[]);
 console.log('PASS: Markdown documents, responsive tables and code, source line controls, stable composer defaults and draft formats; independent appearance switches, compact avatars, 64 line/quote/resource and independent post/reply bubble combinations; post-focused responsive feed, separate composer, retained drafts and guest publishing permission; fixed mobile menu cells across labels, pages and selection; stable clipboard entry across navigation and settings delays; topic dropdown navigation, filtering, keyboard and responsive layout; pagination, retry, search, drafts, navigation races, theme, responsive layout, missing articles; no browser errors');
}finally{await browser.close()}
