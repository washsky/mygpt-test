package forum

// readingStyles keeps the reading layout separate from the embedded application.
const readingStyles = `<style>
main>div,aside,.workspace-hero>div{min-width:0}
.workspace-hero{padding:24px 0 14px;border-bottom:1px solid var(--line);margin-bottom:28px}
.eyebrow{font-size:11px;letter-spacing:.18em;color:var(--accent);font-weight:700;margin-bottom:10px}
.workspace-hero h1{font-size:clamp(28px,4vw,40px);line-height:1.3;overflow-wrap:anywhere}
#site-description{max-width:58ch;overflow-wrap:anywhere}
#feed>.card:first-child{padding:28px}
#feed-status{min-height:28px;margin:12px 0 0}
#feed-status button{margin-left:8px}
.post{position:relative;border-color:var(--line);padding:24px 0;overflow-wrap:anywhere}
.post-title{font-size:21px;font-weight:650;line-height:1.55;margin:7px 0 9px;letter-spacing:-.02em}
.post-link:hover{text-decoration:none}.post-link:hover .post-title{color:var(--accent)}
.post-topic-label{display:inline-block;padding:2px 8px;border-radius:5px;background:var(--tint);color:var(--accent);font-size:11px}
.post .admin-tools{margin-bottom:0}.post-preview{color:var(--muted);line-height:1.85;display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden}
.feed-empty{text-align:center;padding:40px 12px}.feed-empty h3{font-size:22px}.feed-empty p{margin-bottom:22px}
#more{display:block;margin:22px auto 0;min-width:150px}
#detail{padding:clamp(20px,4vw,44px)}
#detail-title{font-size:clamp(26px,3.4vw,38px);line-height:1.45;overflow-wrap:anywhere;margin:10px 0}
#reading-time{margin:0 0 20px}#detail-meta{padding:16px 0;border-block:1px solid var(--line)}
#detail-body{font-size:17px;line-height:1.95;padding:28px 0;tab-size:4}
#detail-body,.reply .body{word-break:normal;overflow-wrap:anywhere}
#detail .inline-image{height:auto;max-width:100%}.attachment{max-width:100%}.attachment>span{overflow-wrap:anywhere;min-width:0}
#editor{scroll-margin-top:24px}#editor>h2{font-size:24px}#post-body{line-height:1.8}
#search-form{padding:10px;background:var(--soft);border:1px solid var(--line);border-radius:12px;flex-wrap:wrap}
#search-query{min-width:120px;flex:1 1 200px}#search-form button{min-height:42px}
input:not([type=checkbox]),select,textarea{min-width:0}button.primary,.topic{min-height:44px}
.topic{overflow-wrap:anywhere}.topic-row{min-width:0}.topic-row .topic{text-overflow:ellipsis;overflow:hidden}
.message{overflow-wrap:anywhere}#detail .back{min-height:44px}
@media(max-width:720px){#feed>.card:first-child{padding:18px}.workspace-hero{padding-top:8px;margin-bottom:20px}aside .card{margin-bottom:16px}#topics{align-items:start}.post-title{font-size:20px}#detail-body{font-size:16px}.topic-row{flex:0 1 auto}.topic-row .topic{max-width:200px}#reply-form .reply-compose-tools{display:block}}
@media print{.workspace-bar,.workspace-crumb,aside,.workspace-hero,.workspace-footer,#feed,#settings,#trash,#post-tools,#reply-form,.admin-tools,.back,dialog{display:none!important}main{display:block!important;width:100%!important;margin:0!important}#detail{border:0!important;box-shadow:none!important;padding:0!important}body{background:white!important;color:black!important}.body{font-size:12pt!important}.inline-image{max-height:14cm}a{color:inherit}}
</style>`
