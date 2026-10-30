// Run: npm install --no-save playwright && node tests/browser.mjs
// Uses the actual embedded HTML/JS with a deterministic API fixture; no Go server required.
import { createRequire } from 'node:module';
import assert from 'node:assert/strict';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || (process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES ? process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES + '/playwright' : 'playwright'));
import { html } from './embedded.mjs';
const browser=await chromium.launch({headless:true,executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH,args:['--no-sandbox']});
const context=await browser.newContext();
const page=await context.newPage();
const errors=[];page.on('pageerror',error=>errors.push(error.message));
const settings={title:'山间札记',description:'记录技术、生活与每一次新的发现。',allow_guest_posts:true,allow_guest_replies:true,enable_search:true};
const topics=[{id:'a',name:'日常记录',description:'生活'},{id:'b',name:'技术笔记',description:'技术'}];
const post=id=>({id,title:'给未来的自己留一份记录 · '+id,author:'访客',topic_id:'a',created_at:'2026-01-01T12:00:00Z',reply_count:0,body:'这一篇笔记记录生活与技术。\n'+('文字与长链接 https://example.com/ '+ 'a'.repeat(120)+'\n').repeat(6),replies:[],attachments:[]});
let listCalls=0,failList=false,slow=false,failDetail=false;
await page.route('http://blog.test/**',async route=>{
 const url=new URL(route.request().url());let body={};
 if(url.pathname==='/')return route.fulfill({contentType:'text/html',body:html});
 if(url.pathname==='/api/forum/settings')body=settings;
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
  body=post(id);
 }else body={items:[]};
 return route.fulfill({contentType:'application/json',body:JSON.stringify(body)});
});
try{
 await page.goto('http://blog.test/');await page.waitForFunction(()=>document.querySelectorAll('.post').length===20);
 assert.equal(await page.locator('#posts').getAttribute('aria-busy'),'false');
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
 await page.locator('#post-topic').selectOption('b');await page.evaluate(()=>loadTopics());assert.equal(await page.locator('#post-topic').inputValue(),'b');
 await page.locator('#post-body').fill('尚未发布的草稿');await page.waitForTimeout(350);await page.reload();await page.waitForFunction(()=>document.querySelector('#post-body').value==='尚未发布的草稿');
 assert.equal(await page.locator('#post-topic').inputValue(),'b');
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
 // Missing articles clear old metadata rather than presenting stale content.
 failDetail=true;await page.evaluate(()=>{location.hash='post/missing'});await page.getByText('暂时无法打开内容',{exact:true}).waitFor();
 assert.equal(await page.locator('#detail-meta').textContent(),'');
 assert.deepEqual(errors,[]);
 console.log('PASS: pagination, retry, search, drafts, navigation races, theme, responsive layout, missing articles; no browser errors');
}finally{await browser.close()}
