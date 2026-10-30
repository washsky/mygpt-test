import { readFileSync } from 'node:fs';

// Expand the Go raw-string constants used to assemble the real forum page.
const constants = {};
for (const file of ['internal/ui/auth.go','internal/ui/live.go','internal/ui/shell.go','internal/forum/reading.go','internal/forum/page.go','internal/auth/page.go','internal/filemanager/page.go','internal/server/server.go']) {
 const source=readFileSync(file,'utf8');
 for(const match of source.matchAll(/^const (\w+) = ((?:`[^`]*`|(?:ui\.)?\w+)(?:\s*\+\s*(?:`[^`]*`|(?:ui\.)?\w+))*)/gm))constants[match[1]]=match[2];
}
function expand(name){
 if(!constants[name])throw Error('Unknown Go constant: '+name);
 return constants[name].split(/(`[^`]*`)/).map(part=>part.startsWith('`')?part.slice(1,-1):part.split('+').map(key=>key.trim().replace(/^ui\./,'')).filter(Boolean).map(expand).join('')).join('');
}
export const html=expand('forumPage');
export const accountHTML=expand('accountPage');
export const source=readFileSync('internal/forum/page.go','utf8');

export const fileHTML=expand('filePage');
export const calculatorHTML=expand('page');
