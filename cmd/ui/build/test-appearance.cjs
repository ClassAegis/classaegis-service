const fs = require('fs');
const http = require('http');
const path = require('path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const root = path.resolve('cmd/ui/dist-tauri');
fs.mkdirSync('out/design-preview', { recursive: true });
const server = http.createServer((req,res) => {
 const file = path.resolve(root, '.' + decodeURIComponent(req.url.split('?')[0] === '/' ? '/index.html' : req.url.split('?')[0]));
 if (!file.startsWith(root + path.sep)) { res.writeHead(403).end(); return; }
 fs.readFile(file,(err,data)=>{ if(err){res.writeHead(404).end();return;} res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(data); });
});
(async()=>{
 await new Promise(resolve=>server.listen(19228,'127.0.0.1',resolve));
 const browser = await chromium.launch({channel:'msedge',headless:true});
 try {
 const page = await browser.newPage({viewport:{width:980,height:660},deviceScaleFactor:1,colorScheme:'dark'});
 const errors=[];page.on('pageerror', e=>errors.push(e.message));
 await page.addInitScript(()=>{ window.ocm={getStatus:async()=>{throw Error('未配置核心服务')},getAppInfo:async()=>({version:'0.1.2'}),listLogs:async()=>({files:[]}),minimizeWindow:()=>{},closeWindow:()=>{}}; });
 await page.goto('http://127.0.0.1:19228');
 const assert = (condition, message) => {if(!condition)throw Error(message);console.log('PASS: '+message)};
 const theme = ()=>page.locator('html').getAttribute('data-theme');
 await page.getByRole('button',{name:'设置',exact:true}).click();
 await page.getByRole('heading',{name:'设置',exact:true}).waitFor();
 assert(await theme()==='dark','default follows dark system');
 await page.getByRole('radio',{name:'浅色模式'}).check();
 assert(await theme()==='light','light mode applies immediately');
 await page.waitForTimeout(250);
 await page.screenshot({path:'out/design-preview/settings-light.png'});
 await page.reload();
 assert(await theme()==='light','explicit choice persists after reload');
 await page.emulateMedia({colorScheme:'dark'});
 assert(await theme()==='light','explicit choice ignores system changes');
 for (const [label,slug] of [['首页','home'],['杀毒','antivirus'],['清理','cleanup'],['还原','freeze'],['弹窗拦截','popup'],['工具箱','tools']]) {
   await page.getByRole('navigation').getByRole('button',{name:label,exact:true}).click();
   await page.waitForTimeout(200);
   assert(await theme()==='light','light theme on '+label);
   if(['杀毒','工具箱','首页'].includes(label)) await page.screenshot({path:`out/design-preview/${slug}-light.png`});
 }
 await page.getByRole('button',{name:'网络诊断',exact:false}).click();
 const dialog=page.getByRole('dialog');await dialog.waitFor();
 assert(await page.locator('.el-dialog').evaluate(el=>getComputedStyle(el).backgroundColor)==='rgb(255, 255, 255)','preview dialog uses light surface');
 await page.getByRole('button',{name:'知道了'}).click();
 await page.getByRole('button',{name:'设置',exact:true}).click();
 await page.getByRole('radio',{name:'浅色模式'}).focus();
 await page.keyboard.press('ArrowRight');
 assert(await theme()==='dark','theme supports keyboard selection');
 await page.getByRole('radio',{name:'深色模式'}).check();
 assert(await theme()==='dark','dark mode applies');
 await page.waitForTimeout(250);
 await page.screenshot({path:'out/design-preview/settings-dark.png'});
 await page.getByRole('radio',{name:'跟随系统'}).check();
 await page.emulateMedia({colorScheme:'light'});
 await page.waitForFunction(()=>document.documentElement.dataset.theme==='light');
 assert(await theme()==='light','system mode reacts to light changes');
 await page.emulateMedia({colorScheme:'dark'});
 await page.waitForFunction(()=>document.documentElement.dataset.theme==='dark');
 assert(await theme()==='dark','system mode reacts to dark changes');
 await page.getByRole('radio',{name:'浅色模式'}).check();
 await page.setViewportSize({width:820,height:560});
 await page.screenshot({path:'out/design-preview/settings-small.png'});
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'minimum window has no horizontal page overflow');
 await page.getByRole('button',{name:'关于',exact:true}).click();
 await page.getByRole('heading',{name:'关于课佑',exact:true}).waitFor();
 await page.screenshot({path:'out/design-preview/about-light.png'});
 if(errors.length)throw Error(errors.join('\n'));
 }finally{await browser.close();server.close();}
})().catch(e=>{console.error(e);server.close();process.exitCode=1;});
