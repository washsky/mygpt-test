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
.post{position:relative;border-color:var(--line);padding:22px 0;overflow-wrap:anywhere}
.post-topic-label{display:inline-block;max-width:100%;padding:1px 7px;border-radius:5px;background:var(--soft);color:var(--muted);font-size:11px;line-height:1.7}
.post-title{font-size:23px;font-weight:700;line-height:1.5;margin:7px 0 12px;letter-spacing:-.02em;color:var(--ink)}
.post-link{color:var(--ink)}.post-link:hover{text-decoration:none}.post-link:hover .post-title{color:var(--accent)}
.post-footer{display:flex;gap:8px 20px;align-items:center;justify-content:space-between;flex-wrap:wrap}
.post-meta{display:flex;align-items:center;gap:2px 12px;flex:1;flex-wrap:wrap;min-width:0;font-size:12px;line-height:1.8}
.post-meta time{white-space:nowrap}
.post-actions{display:flex;align-items:center;gap:14px;margin-left:auto}
.post-reply-count{font-size:12px;white-space:nowrap}
.post-actions .admin-button{font-size:12px;line-height:1.8;padding:3px 10px;min-height:32px}
.post-preview{color:var(--muted);font-size:14px;line-height:1.8;margin:0 0 12px;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
.feed-empty{text-align:center;padding:48px 12px}.feed-empty h3{font-size:22px}.feed-empty p{margin-bottom:22px}
#more{display:block;margin:12px auto 18px;min-width:150px}
#editor,#settings,#trash{max-width:900px;margin-inline:auto;padding:clamp(20px,4vw,36px)}
#editor>h2{font-size:27px;margin:0 0 8px}
.draft-status{color:var(--muted);font-size:12px;margin:0 0 22px}
#post-form{display:grid;gap:4px}#post-body{min-height:300px;line-height:1.8}
#settings-form{max-width:720px}
#settings-form>label:has(input[type=checkbox]){padding:12px;background:var(--soft);border-radius:10px}
#settings h3{border-top:1px solid var(--line);padding-top:22px;margin-top:26px}
#trash-list h3{padding:12px 16px;background:var(--soft);border-radius:10px;margin-top:28px}
.trash-entry{padding:18px 12px}.trash-entry strong{font-size:17px}
.topic{color:var(--ink);overflow-wrap:anywhere;min-height:44px}.topic:hover,.topic.active{background:var(--tint);color:var(--accent)}
.topic-row{min-width:0}.topic-row .topic{text-overflow:ellipsis;overflow:hidden}
#topics .admin-button{font-size:11px;padding:4px}
#detail{max-width:940px;margin-inline:auto;padding:clamp(20px,4vw,44px)}
#detail-title{font-size:clamp(27px,3.4vw,38px);line-height:1.45;overflow-wrap:anywhere;margin:10px 0}
#reading-time{margin:0 0 16px}
#detail-meta{padding:16px 0;border-block:1px solid var(--line)}
#detail-body{font-size:18px;line-height:1.95;padding:24px 0;tab-size:4}
#detail-body,.reply .body{word-break:normal;overflow-wrap:anywhere}
#detail .inline-image{height:auto;max-width:100%}.attachment{max-width:100%}.attachment>span{overflow-wrap:anywhere;min-width:0}.attachment a{white-space:nowrap}
.reply{border-color:var(--line)}
input:not([type=checkbox]),select,textarea{min-width:0}button.primary{min-height:44px}
.message{overflow-wrap:anywhere}.message:empty{display:none}
#editor .back,#detail .back{min-height:44px}
@media(max-width:760px){
 main{width:calc(100% - 24px);margin-top:12px}
 .forum-heading{display:block;margin-bottom:10px;padding-inline:4px}
 #site-title{font-size:17px}#site-description{font-size:12px;margin-top:2px}
 #feed>.feed-card{padding:14px 16px 4px}
 .feed-toolbar{gap:12px}.feed-heading{flex-basis:100%}
 #feed-title{font-size:22px}#compose-link{font-size:12px;padding:7px 11px}
 #search-form{flex-basis:100%}
 #feed-status{min-height:20px;margin-top:8px}
 .post{padding:16px 0}.post-title{font-size:21px;margin:6px 0 10px}
 #posts>.post:first-child{padding-top:10px}
 .post-footer{gap:6px 12px}.post-meta{font-size:11px}
 .post-actions{gap:12px}.post-reply-count{font-size:11px}
 #detail-body{font-size:17px}
 #reply-form .reply-compose-tools{display:block}
}
@media print{.workspace-bar,.workspace-crumb,.workspace-footer,#feed,#editor,#settings,#trash,#post-tools,#reply-form,.admin-tools,.back,dialog{display:none!important}main{display:block!important;width:100%!important;margin:0!important}#detail{max-width:none;border:0!important;box-shadow:none!important;padding:0!important}body{background:white!important;color:black!important}.body{font-size:12pt!important}.inline-image{max-height:14cm}a{color:inherit}}
</style>`
