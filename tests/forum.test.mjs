import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {html,source} from './embedded.mjs';

class Element {
 constructor(tag='div',text='',className=''){this.tag=tag;this.textContent=text;this.className=className;this.children=[];this.attributes={};this.value='';this.options=[];this.classes=new Set();this.classList={add:x=>this.classes.add(x),remove:x=>this.classes.delete(x),toggle:(x,on)=>on?this.classes.add(x):this.classes.delete(x)};}
 append(...nodes){for(const node of nodes){if(node.tag==='fragment')this.children.push(...node.children);else this.children.push(node)}}
 prepend(...nodes){this.children.unshift(...nodes)}
 replaceChildren(...nodes){this.children=[];this.textContent='';this.append(...nodes)}
 setAttribute(key,value){this.attributes[key]=value}
 focus(){}
}
function fixture(){
 const elements=new Map();const $=id=>{if(!elements.has(id))elements.set(id,new Element());return elements.get(id)};
 const make=(...args)=>new Element(...args);
 const requests=[];
 const context=vm.createContext({$,make,document:{title:'',createDocumentFragment:()=>make('fragment'),createTextNode:text=>make('text',text),getElementById:id=>$('#'+id)},window:{getSelection:()=>''},location:{origin:'http://blog.test',hash:'#post/1'},AbortController,setTimeout,clearTimeout,console,offset:0,currentTopic:'',currentPost:'',searchQuery:'',allTopics:[],settings:{title:'测试博客',allow_guest_replies:true},attachmentNotice:{id:'',text:''},date:value=>value,copyButton:()=>make('button'),adminButton:(label,onclick)=>Object.assign(make('button',label),{onclick}),err:async r=>{if(!r.ok)throw Error('failed');return r.json()},fetch:(url,options)=>new Promise((resolve,reject)=>requests.push({url,options,resolve:items=>resolve({ok:true,json:async()=>({items})}),reject})),api:()=>{},show:()=>{},key:()=>'',visitorInfo:()=>make('div'),resourceContent:(target,body)=>target.textContent=body,attachments:()=>{},compactActions:()=>{}});
 vm.runInContext(source.slice(source.indexOf('let feedRequest='),source.indexOf('async function upload(')),context);
 return {context,$,requests,run:code=>vm.runInContext(code,context)};
}
const posts=Array.from({length:20},(_,i)=>({id:String(i),title:'笔记'+i,author:'作者',created_at:'2026-01-01',reply_count:0}));

test('all assembled inline scripts parse',()=>{
 let count=0;for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g)){new vm.Script(match[1]);count++}assert.ok(count>=4);
 assert.ok(html.includes('id="feed-status"'));assert.ok(html.includes('id="main-content"'));
});
test('pagination coalesces duplicate clicks and appends exactly once',async()=>{
 const f=fixture();let p=f.run('loadPosts(false)');f.requests[0].resolve(posts);await p;
 p=f.run('loadPosts(true)');await f.run('loadPosts(true)');assert.equal(f.requests.length,2);assert.match(f.requests[1].url,/offset=20/);
 f.requests[1].resolve(posts);await p;assert.equal(f.$('#posts').children.length,40);assert.equal(f.$('#posts').attributes['aria-busy'],'false');
});
test('new list requests abort and ignore stale results',async()=>{
 const f=fixture();const old=f.run('loadPosts(false)');const fresh=f.run('searchQuery="new";loadPosts(false)');assert.equal(f.requests[0].options.signal.aborted,true);
 f.requests[1].resolve(posts.slice(0,1));await fresh;f.requests[0].resolve(posts);await old;
 assert.equal(f.$('#posts').children.length,1);assert.equal(f.run('offset'),1);
});
test('failed next page retains posts and retries the same offset',async()=>{
 const f=fixture();let p=f.run('loadPosts(false)');f.requests[0].resolve(posts);await p;
 p=f.run('loadPosts(true)');f.requests[1].reject(Error('offline'));await p;assert.equal(f.$('#posts').children.length,20);
 const retry=f.$('#feed-status').children.find(el=>el.tag==='button');p=retry.onclick();assert.match(f.requests[2].url,/offset=20/);f.requests[2].resolve([]);await p;
 assert.equal(f.$('#more').disabled,false);assert.equal(f.run('offset'),20);
});
test('empty search gives a clear-search action',async()=>{
 const f=fixture();let cleared=false;f.$('#clear-search').click=()=>cleared=true;
 const p=f.run('searchQuery="missing";loadPosts(false)');f.requests[0].resolve([]);await p;
 const action=f.$('#posts').children[0].children.find(el=>el.tag==='a');action.onclick({preventDefault(){}});assert.equal(cleared,true);
});
test('out-of-order article responses cannot replace the active article',async()=>{
 const f=fixture(),responses=[];f.context.api=()=>new Promise(resolve=>responses.push(resolve));
 const first=f.run('showPost("1")');const second=f.run('location.hash="#post/2";showPost("2")');
 const article=id=>({id,title:'标题'+id,body:'正文',author:'作者',created_at:'2026-01-01',replies:[],attachments:[]});
 responses[1](article('2'));await second;responses[0](article('1'));await first;
 assert.equal(f.$('#detail-title').textContent,'标题2');assert.equal(f.context.document.title,'标题2 · 测试博客');
});
test('restoring a draft ignores a deleted topic and keeps its body',()=>{
 const f=fixture();f.$('#post-topic').value='valid';f.$('#post-topic').options=[{value:'valid'}];
 f.context.localStorage={getItem:()=>JSON.stringify({'post-topic':'deleted','post-body':'未发布正文'})};f.context.sessionStorage={getItem:()=>null};
 f.run(source.slice(source.indexOf('const draftFields='),source.indexOf('let draftTimer;')));f.run('restoreDraft()');
 assert.equal(f.$('#post-topic').value,'valid');assert.equal(f.$('#post-body').value,'未发布正文');
});
