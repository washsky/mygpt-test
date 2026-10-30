// Real-server smoke test, isolated from the user's running instance and data.
// MYGPT_TEST_BINARY=/path/to/binary PLAYWRIGHT_MODULE=/path/to/playwright node tests/auth-browser.mjs
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {spawn} from 'node:child_process';
import {createServer} from 'node:net';
const require=createRequire(import.meta.url);
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
assert.ok(process.env.MYGPT_TEST_BINARY,'Set MYGPT_TEST_BINARY to a built test binary');
const port=await new Promise(resolve=>{const s=createServer();s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>resolve(p))})});
const root=await mkdtemp(join(tmpdir(),'mygpt-auth-')),base='http://127.0.0.1:'+port;
const server=spawn(resolve(process.env.MYGPT_TEST_BINARY),['-addr','127.0.0.1:'+port,'-data-dir',root],{stdio:'ignore'});
let browser;
try{
 for(let i=0;i<100;i++){try{if((await fetch(base+'/healthz')).ok)break}catch(_){}if(i===99)throw Error('Test server did not start');await new Promise(r=>setTimeout(r,100))}
 browser=await chromium.launch({headless:true,executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH,args:['--no-sandbox']});
 const context=await browser.newContext(),page=await context.newPage(),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.goto(base+'/account');await page.locator('#register').waitFor();
 await page.locator('#register [name=username]').fill('browseruser');await page.locator('#register [name=password]').fill('test browser password');await page.locator('#register [name=confirm]').fill('test browser password');await page.locator('#register button').click();
 await page.locator('#status').filter({hasText:'账户已创建'}).waitFor();
 await page.locator('#login [name=username]').fill('browseruser');await page.locator('#login [name=password]').fill('test browser password');await page.locator('#login button').click();
 await page.locator('#signed-in').waitFor();assert.match(await page.locator('#identity').textContent(),/browseruser/);
 await page.goto(base+'/');await page.locator('#post-author[readonly]').waitFor();
 await page.locator('#post-title').fill('账户笔记');await page.locator('#post-body').fill('浏览器实际发布');await page.locator('#post-form button[type=submit]').click();
 await page.locator('#detail-title').filter({hasText:'账户笔记'}).waitFor();assert.match(await page.locator('#detail-meta').textContent(),/browseruser · 注册用户/);
 await page.locator('#reply-body').fill('账户回复');await page.locator('#reply-form button[type=submit]').click();await page.locator('.reply').filter({hasText:'账户回复'}).waitFor();
 await page.goto(base+'/files');await page.locator('#workspace-account').filter({hasText:'browseruser'}).waitFor();
 const input=page.locator('input[type=file]');await input.setInputFiles({name:'account-upload.txt',mimeType:'text/plain',buffer:Buffer.from('authenticated upload')});
 // The file page uses XHR; its CSRF header must be tested separately from fetch.
 const response=page.waitForResponse(r=>r.url()===base+'/api/files'&&r.request().method()==='POST');await page.locator('#submit').click();assert.equal((await response).status(),201);
 await page.goto(base+'/account');await page.locator('#logout').click();await page.locator('#signed-out').waitFor();
 await page.goto(base+'/');await page.locator('#posts[aria-busy=false]').waitFor();assert.equal(await page.locator('#post-author').getAttribute('readonly'),null);
 const token=(await readFile(join(root,'config/admin-token'),'utf8')).trim();
 await page.goto(base+'/account');await page.locator('#setup summary').click();await page.locator('#setup-form [name=token]').fill(token);await page.locator('#setup-form [name=username]').fill('siteowner');await page.locator('#setup-form [name=password]').fill('owner browser password');await page.locator('#setup-form [name=confirm]').fill('owner browser password');await page.locator('#setup-form button').click();await page.locator('#status').filter({hasText:'账户已创建'}).waitFor();
 await page.locator('#login [name=username]').fill('siteowner');await page.locator('#login [name=password]').fill('owner browser password');await page.locator('#login button').click();await page.locator('#management').waitFor();await page.locator('#users button').filter({hasText:'停用'}).click();await page.getByText('browseruser · 已停用',{exact:true}).waitFor();
 await page.goto(base+'/#settings');await page.locator('#guest-posts').uncheck();await page.locator('#guest-uploads').uncheck();await page.locator('#settings-form button[type=submit]').click();await page.locator('#settings-message').filter({hasText:'已保存'}).waitFor();
 assert.deepEqual(errors,[]);await context.close();console.log('Auth browser smoke passed: registration, login, member post/reply, XHR upload, logout, admin setup, user disable, settings.');
}finally{
 if(browser)await browser.close();server.kill('SIGTERM');await new Promise(r=>server.once('exit',r));await rm(root,{recursive:true,force:true});
}
