// Real-server smoke test, isolated from the user's running instance and data.
// MYGPT_TEST_BINARY=/path/to/binary PLAYWRIGHT_MODULE=/path/to/playwright node tests/auth-browser.mjs
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {spawn} from 'node:child_process';
import {createServer} from 'node:net';
const require=createRequire(import.meta.url);
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
assert.ok(process.env.MYGPT_TEST_BINARY,'Set MYGPT_TEST_BINARY to a built test binary');
const port=await new Promise(resolve=>{const s=createServer();s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>resolve(p))})});
const memberPassword='长密码'.repeat(1000),ownerPassword='x';
const root=await mkdtemp(join(tmpdir(),'mygpt-auth-')),base='http://127.0.0.1:'+port;
const server=spawn(resolve(process.env.MYGPT_TEST_BINARY),['-addr','127.0.0.1:'+port,'-data-dir',root],{stdio:'ignore'});
let browser;
try{
 for(let i=0;i<100;i++){try{if((await fetch(base+'/healthz')).ok)break}catch(_){}if(i===99)throw Error('Test server did not start');await new Promise(r=>setTimeout(r,100))}
 browser=await chromium.launch({headless:true,executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH,args:['--no-sandbox']});
 const context=await browser.newContext(),page=await context.newPage(),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.goto(base+'/account');await page.locator('#registration-note').filter({hasText:'第一个注册成功'}).waitFor();
 await page.locator('#register [name=username]').fill('siteowner');await page.locator('#register [name=password]').fill(ownerPassword);await page.locator('#register [name=confirm]').fill(ownerPassword);await page.locator('#register button').click();await page.locator('#status').filter({hasText:'UID 1'}).waitFor();
 await page.locator('#login [name=username]').fill('siteowner');await page.locator('#login [name=password]').fill(ownerPassword);await page.locator('#login button').click();await page.locator('#identity').filter({hasText:'UID 1 · 管理员'}).waitFor();
 assert.equal(await page.locator('#setup').count(),0);
 await page.goto(base+'/');await page.locator('#workspace-account').filter({hasText:'siteowner'}).waitFor();await page.locator('#workspace-forum').click();await page.locator('#add-topic').click();await page.locator('#topic-name').fill('下拉主题');await page.locator('#topic-description').fill('导航里的主题管理');await page.locator('#topic-form button[type=submit]').click();await page.locator('#topic-message').filter({hasText:'已创建主题'}).waitFor();assert.equal(await page.locator('#post-topic option').filter({hasText:'下拉主题'}).count(),1);
 await page.goto(base+'/account');await page.locator('#logout').click();await page.locator('#register').waitFor();
 await page.locator('#register [name=username]').fill('browseruser');await page.locator('#register [name=password]').fill(memberPassword);await page.locator('#register [name=confirm]').fill(memberPassword);await page.locator('#register button').click();
 await page.locator('#status').filter({hasText:'账户已创建'}).waitFor();
 await page.locator('#login [name=username]').fill('browseruser');await page.locator('#login [name=password]').fill(memberPassword);await page.locator('#login button').click();
 await page.locator('#signed-in').waitFor();assert.match(await page.locator('#identity').textContent(),/browseruser/);
 await page.goto(base+'/');await page.locator('#compose-link').click();await page.locator('#post-author[readonly]').waitFor();
 await page.locator('#post-title').fill('账户笔记');await page.locator('#post-body').fill('浏览器实际发布');await page.locator('#post-form button[type=submit]').click();
 await page.locator('#detail-title').filter({hasText:'账户笔记'}).waitFor();assert.match(await page.locator('#detail-meta').textContent(),/browseruser · UID 2 · 注册用户/);
 await page.locator('#reply-body').fill('账户回复');await page.locator('#reply-form button[type=submit]').click();await page.locator('.reply').filter({hasText:'账户回复'}).waitFor();
 await page.goto(base+'/files');await page.locator('#workspace-account').filter({hasText:'browseruser'}).waitFor();
 const input=page.locator('input[type=file]');await input.setInputFiles({name:'account-upload.txt',mimeType:'text/plain',buffer:Buffer.from('authenticated upload')});
 // The file page uses XHR; its CSRF header must be tested separately from fetch.
 const response=page.waitForResponse(r=>r.url()===base+'/api/files'&&r.request().method()==='POST');await page.locator('#submit').click();assert.equal((await response).status(),201);
 await page.goto(base+'/account');await page.locator('#logout').click();await page.locator('#signed-out').waitFor();
 await page.goto(base+'/');await page.locator('#posts[aria-busy=false]').waitFor();assert.equal(await page.locator('#post-author').getAttribute('readonly'),null);
 await page.goto(base+'/account');
 await page.locator('#login [name=username]').fill('siteowner');await page.locator('#login [name=password]').fill(ownerPassword);await page.locator('#login button').click();await page.locator('#management').waitFor();await page.locator('#users button').filter({hasText:'停用'}).click();await page.getByText('browseruser · UID 2 · 已停用',{exact:true}).waitFor();
 await page.goto(base+'/#settings');await page.locator('#guest-posts').uncheck();await page.locator('#guest-uploads').uncheck();await page.locator('#settings-form button[type=submit]').click();await page.locator('#settings-message').filter({hasText:'已保存'}).waitFor();
 // Saving each appearance combination must preserve every existing setting.
 const beforeAppearance=await page.evaluate(()=>fetch('/api/forum/settings').then(r=>r.json()));
 for(const avatars of [false,true])for(const bubble of [false,true])for(const replyBubble of [false,true]){
  await page.locator('#list-avatars').setChecked(avatars);await page.locator('#body-bubble').setChecked(bubble);await page.locator('#reply-bubble').setChecked(replyBubble);
  const saved=page.waitForResponse(r=>r.url()===base+'/api/forum/settings'&&r.request().method()==='PUT');
  await page.locator('#settings-form button[type=submit]').click();assert.equal((await saved).status(),200);
  await page.reload();await page.locator('#posts[aria-busy=false]').waitFor({state:'attached'});
  assert.equal(await page.locator('#list-avatars').isChecked(),avatars);assert.equal(await page.locator('#body-bubble').isChecked(),bubble);assert.equal(await page.locator('#reply-bubble').isChecked(),replyBubble);
  const actual=await page.evaluate(()=>fetch('/api/forum/settings').then(r=>r.json()));
  assert.deepEqual(actual,{...beforeAppearance,enable_list_avatars:avatars,enable_body_bubble:bubble,enable_reply_bubble:replyBubble});
 }
 assert.deepEqual(errors,[]);await context.close();console.log('Auth browser smoke passed: registration, login, member post/reply, XHR upload, logout, first-user admin, sequential UIDs, user disable, settings, short and long passwords, dropdown topic creation.');
}finally{
 if(browser)await browser.close();server.kill('SIGTERM');await new Promise(r=>server.once('exit',r));await rm(root,{recursive:true,force:true});
}
