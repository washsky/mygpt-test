package forum

// readingStyles keeps the forum layout and reading view separate from application logic.
const readingStyles = `<style>
main{width:min(1080px,calc(100% - 48px));margin:20px auto 28px}
main>div{min-width:0}
.forum-heading{display:flex;align-items:baseline;gap:8px 18px;flex-wrap:wrap;margin:0 0 16px;padding:0 2px}
#site-title{font-size:20px;line-height:1.5;font-weight:650;margin:0;overflow-wrap:anywhere}
#site-description{color:var(--muted);font-size:13px;line-height:1.6;margin:0;min-width:0;max-width:68ch;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;overflow-wrap:anywhere}
#feed>.feed-card{padding:24px 30px 10px}
.feed-toolbar{display:flex;align-items:center;justify-content:space-between;gap:14px 24px;flex-wrap:wrap}
.feed-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;flex:1 1 260px;min-width:0}
#feed-title{font-size:25px;line-height:1.4;margin:0;overflow-wrap:anywhere}
#compose-link{font-size:13px;min-height:42px;padding:8px 13px;flex-shrink:0}
#search-form{flex:0 1 390px;min-width:0;gap:6px;margin:0;align-items:center;flex-wrap:nowrap}
#search-query{min-width:0;flex:1 1 120px;padding:8px 10px;height:42px}
#search-form button{min-height:42px;flex-shrink:0;padding:7px 11px;font-size:13px;margin:0}
#feed-status{min-height:26px;margin:12px 0 0;font-size:12px}
#feed-status button{margin-left:8px}
.post{position:relative;display:grid;grid-template-columns:minmax(0,1fr);gap:10px;border-color:var(--line);padding:12px 0;overflow-wrap:anywhere}
.post-main{min-width:0}.post-avatar{display:none;align-self:center;margin:0;width:32px;height:32px;font-size:13px}
#posts[data-avatars=true] .post{grid-template-columns:32px minmax(0,1fr)}
#posts[data-avatars=true] .post-avatar{display:grid}
.post-topic-label{display:inline-block;max-width:100%;padding:0 6px;border-radius:4px;background:var(--soft);color:var(--muted);font-size:11px;line-height:1.8}
.post-title{font-size:22px;font-weight:650;line-height:1.4;margin:0 0 4px;letter-spacing:-.02em;color:var(--ink)}
.post-link{color:var(--ink)}.post-link:hover{text-decoration:none}.post-link:hover .post-title{color:var(--accent)}
.post-footer{display:flex;gap:6px 12px;align-items:center;justify-content:space-between}
.post-meta{display:flex;align-items:center;gap:2px 8px;flex:1;flex-wrap:wrap;min-width:0;font-size:12px;line-height:1.8}
.post-meta time{white-space:nowrap}
.post-actions{display:flex;align-items:center;flex-shrink:0;gap:10px;margin-left:auto}
.post-reply-count{font-size:12px;white-space:nowrap}
.post-actions .admin-button{font-size:12px;line-height:1.8;padding:3px 8px;min-height:30px}
.post-preview{color:var(--muted);font-size:14px;line-height:1.7;margin:6px 0;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
.feed-empty{text-align:center;padding:48px 12px}.feed-empty h3{font-size:22px}.feed-empty p{margin-bottom:22px}
#more{display:block;margin:12px auto 18px;min-width:150px}
#editor,#settings,#trash{max-width:900px;margin-inline:auto;padding:clamp(20px,4vw,36px)}
#editor>h2{font-size:27px;margin:0 0 8px}
.draft-status{color:var(--muted);font-size:12px;margin:0 0 22px}
#post-form{display:grid;gap:4px}#post-body{min-height:300px;line-height:1.8}
#settings-form{max-width:720px}.setting-help{margin:6px 0 16px;padding-inline:12px}
.appearance-options{min-width:0;margin:16px 0 0;padding:8px 12px 12px;border:1px solid var(--line);border-radius:10px}
.appearance-options legend{font-weight:650;padding:0 5px}.appearance-options label{display:inline-flex;align-items:center;gap:7px;margin:4px 18px 0 0;font-size:14px;font-weight:500}
#settings-form>label:has(input[type=checkbox]){padding:12px;background:var(--soft);border-radius:10px}
#settings h3{border-top:1px solid var(--line);padding-top:22px;margin-top:26px}
#trash-list h3{padding:12px 16px;background:var(--soft);border-radius:10px;margin-top:28px}
.trash-entry{padding:18px 12px}.trash-entry strong{font-size:17px}
.topic{color:var(--ink);overflow-wrap:anywhere;min-height:44px}.topic:hover,.topic.active{background:var(--tint);color:var(--accent)}
.topic-row{min-width:0}.topic-row .topic{text-overflow:ellipsis;overflow:hidden}
#topics .admin-button{font-size:11px;padding:4px}
#detail{max-width:940px;margin-inline:auto;padding:clamp(20px,4vw,44px)}
#detail-title{font-size:clamp(27px,3.4vw,36px);line-height:1.4;overflow-wrap:anywhere;margin:8px 0 14px}
#detail-title:focus{outline:none}
#detail-meta{padding:0;border:0}
.visitor-info>div{min-width:0;overflow-wrap:anywhere}
#detail-meta .visitor-info{align-items:start}
#detail-meta .visitor-info strong{font-size:13px;font-weight:600}
.detail-topbar{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:10px}
.detail-topbar .back{min-height:36px!important;padding:0}
#reading-time{margin:8px 0 0;font-size:12px}
#post-tools{min-width:0}#post-tools .admin-tools{margin:0}
#post-tools .admin-button,#post-tools .action-menu>summary{font-size:12px}
#article-content{margin:18px 0 24px;padding:20px 0;border-block:1px solid var(--line);min-width:0}
#detail[data-body-bubble=true] #article-content{padding:20px 22px;border:1px solid var(--line);border-radius:14px;background:var(--soft)}
#detail-body{font-size:18px;line-height:1.9;padding:0;tab-size:4}
#detail-files:empty{display:none}
.replies-heading{font-size:18px;margin:28px 0 8px}
#detail-body,.reply .body{word-break:normal;overflow-wrap:anywhere}
#detail .inline-image{height:auto;max-width:100%}.attachment{max-width:100%}.attachment>span{overflow-wrap:anywhere;min-width:0}.attachment a{white-space:nowrap}
.reply{border-color:var(--line)}
.reply-content{min-width:0;margin:14px 0}.reply-content>.body{margin:0}
.reply-files:empty{display:none}
#detail[data-reply-bubble=true] .reply-content{padding:16px 20px;border:1px solid var(--line);border-radius:14px;background:var(--soft)}
input:not([type=checkbox]),select,textarea{min-width:0}button.primary{min-height:44px}
.message{overflow-wrap:anywhere}.message:empty{display:none}
#editor .back,#detail .back{min-height:44px}

.format-choice{display:flex;align-items:center;gap:8px;font-size:13px;font-weight:500;margin:6px 0 8px}.format-choice input{margin:0}
.markdown-help{margin:6px 0 10px}
.markdown-rendered{white-space:normal;overflow-wrap:anywhere;font-size:16px;line-height:1.8;min-width:0}
.markdown-rendered>:first-child{margin-top:0}.markdown-rendered>:last-child{margin-bottom:0}
.markdown-rendered h1{font-size:28px}.markdown-rendered h2{font-size:24px}.markdown-rendered h3{font-size:20px}.markdown-rendered h4,.markdown-rendered h5,.markdown-rendered h6{font-size:17px}
.markdown-rendered h1,.markdown-rendered h2,.markdown-rendered h3,.markdown-rendered h4,.markdown-rendered h5,.markdown-rendered h6{margin:1.2em 0 .5em;line-height:1.45;letter-spacing:0}
.markdown-rendered p,.markdown-rendered ul,.markdown-rendered ol,.markdown-rendered dl{margin:.7em 0}
.markdown-rendered ul,.markdown-rendered ol{padding-left:1.7em}.markdown-rendered li>p{margin:.25em 0}
.markdown-rendered blockquote{margin:12px 0;padding:1px 14px;border-left:3px solid var(--line);color:var(--muted)}
.markdown-rendered pre{max-width:100%;overflow:auto;white-space:pre;tab-size:4;padding:14px;border:1px solid var(--line);border-radius:9px;background:var(--soft);font:13px/1.7 ui-monospace,monospace}
.markdown-rendered code{padding:2px 5px;border-radius:4px;background:var(--soft);font-family:ui-monospace,monospace;font-size:.9em}
.markdown-rendered pre code{padding:0;background:transparent;font-size:inherit}
.markdown-table-wrap{max-width:100%;overflow:auto;margin:12px 0;border:1px solid var(--line);border-radius:8px}
.markdown-rendered table{border-collapse:collapse;min-width:100%;width:max-content;font-size:14px}
.markdown-rendered th,.markdown-rendered td{border:1px solid var(--line);padding:7px 10px;text-align:left}.markdown-rendered th{background:var(--soft)}
.markdown-rendered th[align=right],.markdown-rendered td[align=right]{text-align:right}.markdown-rendered th[align=center],.markdown-rendered td[align=center]{text-align:center}
.markdown-rendered img{display:block;max-width:100%;height:auto;max-height:70vh;object-fit:contain;margin:12px 0;border-radius:8px}
.markdown-rendered hr{border:0;border-top:1px solid var(--line);margin:20px 0}
.markdown-rendered input[type=checkbox]{width:auto;min-height:0;margin-right:6px;vertical-align:middle}
.markdown-source-tools{margin-top:18px;padding-top:10px;border-top:1px solid var(--line);white-space:normal;font-size:13px}
.markdown-source-tools>summary{cursor:pointer;color:var(--muted);min-height:36px;line-height:36px}
.markdown-source-lines{font-size:13px;line-height:1.8}.markdown-source-lines .line-number{opacity:1}
@media(max-width:760px){
 main{width:calc(100% - 24px);margin-top:12px}
 .forum-heading{display:block;margin-bottom:10px;padding-inline:4px}
 #site-title{font-size:17px}#site-description{font-size:12px;margin-top:2px}
 #feed>.feed-card{padding:14px 16px 4px}
 .feed-toolbar{gap:12px}.feed-heading{flex-basis:100%}
 #feed-title{font-size:22px}#compose-link{font-size:12px;padding:7px 11px}
 #search-form{flex-basis:100%}
 #feed-status{min-height:20px;margin-top:8px}
 .post{padding:11px 0;gap:9px}.post-title{font-size:20px;margin:0 0 3px;line-height:1.4}
 .post-footer{gap:6px 8px}.post-meta{font-size:11px;gap:2px 7px}
 .post-actions{gap:7px}.post-reply-count{font-size:11px}
 #detail{padding:16px}
 #detail[data-body-bubble=true] #article-content{padding:16px 12px}
 #detail-body{font-size:17px}
 #detail[data-reply-bubble=true] .reply-content{padding:14px 12px}
 #article-content .content-line,.reply-content .content-line{gap:6px}
 #article-content .line-number,.reply-content .line-number{flex-basis:2em}
 #post-tools .admin-button{padding:5px 8px}
 #reply-form .reply-compose-tools{display:block}
}
@media print{.workspace-bar,.workspace-crumb,.workspace-footer,#feed,#editor,#settings,#trash,#post-tools,#reply-form,.admin-tools,.back,dialog{display:none!important}main{display:block!important;width:100%!important;margin:0!important}#detail{max-width:none;border:0!important;box-shadow:none!important;padding:0!important}#detail #article-content,#detail .reply-content{background:transparent!important;border:0!important;padding:0!important}body{background:white!important;color:black!important}.body{font-size:12pt!important}.inline-image{max-height:14cm}a{color:inherit}}
</style>`
