package ui

// Auth keeps credentials out of browser storage. CSRF values are held in memory.
const Auth = `<script>
(()=>{const originalFetch=window.fetch.bind(window);let state={user:null,csrf_token:''};window.workspaceAuth=state;
const ready=originalFetch('/api/auth/me').then(async r=>{if(!r.ok)throw Error('无法验证登录状态，请刷新后重试');state=await r.json();window.workspaceAuth=state;return state});
// Pages can read publicly even when session discovery temporarily fails; writes wait.
ready.catch(()=>{});window.workspaceAuthReady=ready;
window.fetch=async(input,options={})=>{const url=new URL(typeof input==='string'?input:input.url,location.href),method=(options.method||(input instanceof Request?input.method:'GET')).toUpperCase();
 if(url.origin===location.origin&&url.pathname.startsWith('/api/')&&!['GET','HEAD','OPTIONS'].includes(method)){await ready;const headers=new Headers(options.headers||(input instanceof Request?input.headers:undefined));if(state.csrf_token)headers.set('X-CSRF-Token',state.csrf_token);options={...options,headers}}
 return originalFetch(input,options)};
addEventListener('DOMContentLoaded',()=>{const link=document.getElementById('workspace-account');if(link)ready.then(s=>{link.textContent=s.user?s.user.username+' · UID '+s.user.uid+' · '+(s.user.role==='admin'?'管理员':'账户'):'登录 / 注册'}).catch(()=>{link.textContent='账户（连接失败）'})});
})();
</script>`
