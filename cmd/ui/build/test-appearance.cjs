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
 for (const [label,slug] of [['首页','home'],['杀毒','antivirus'],['清理','cleanup'],['还原','freeze'],['弹窗拦截','popup'],['软件管理','software'],['工具箱','tools']]) {
   await page.getByRole('navigation').getByRole('button',{name:label,exact:true}).click();
   await page.waitForTimeout(200);
   assert(await theme()==='light','light theme on '+label);
   if(['杀毒','弹窗拦截','软件管理','工具箱','首页'].includes(label)) await page.screenshot({path:`out/design-preview/${slug}-light.png`});
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
 for (const [label,slug] of [['首页','home'],['杀毒','antivirus'],['弹窗拦截','popup'],['软件管理','software']]) {
   await page.getByRole('navigation').getByRole('button',{name:label,exact:true}).click();
   await page.waitForTimeout(200);
   await page.screenshot({path:`out/design-preview/${slug}-dark.png`});
 }
 await page.getByRole('button',{name:'查看已装软件',exact:true}).click();
 await page.getByRole('dialog').waitFor();
 assert(await page.getByRole('dialog').innerText().then(text=>text.includes('预览')),'software management opens preview explanation');
 await page.getByRole('button',{name:'知道了'}).click();
 await page.getByRole('button',{name:'设置',exact:true}).click();
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
 assert(await page.evaluate(()=>{
   const nav=document.querySelector('.rail-nav');
   const bottom=document.querySelector('.rail-bottom');
   return nav.scrollHeight<=nav.clientHeight+1 && nav.getBoundingClientRect().bottom<=bottom.getBoundingClientRect().top;
 }),'all seven navigation entries fit above bottom actions at minimum size');
 await page.getByRole('button',{name:'关于',exact:true}).click();
 await page.getByRole('heading',{name:'关于课佑',exact:true}).waitFor();
 await page.screenshot({path:'out/design-preview/about-light.png'});
 await page.getByRole('navigation').getByRole('button',{name:'首页',exact:true}).click();
 await page.getByRole('heading',{name:'暂时无法读取设备状态',exact:true}).waitFor();
 await page.screenshot({path:'out/design-preview/home-small.png'});
 assert(await page.locator('.overview-home').evaluate(el=>el.scrollWidth<=el.clientWidth),'home fits minimum window width');
 await page.evaluate(()=>{window.ocm.getStatus=async()=>({ok:true,data:{sn:'TEST-DEVICE-001',agentVersion:'0.1.2',connected:true,abilityAlive:true}})});
 await page.getByRole('button',{name:'全面体检',exact:true}).click();
 await page.getByRole('heading',{name:'设备运行状态良好',exact:true}).waitFor();
 assert(await page.locator('.check-result.ok').count()===3,'checkup displays three healthy services from status response');
 assert(await page.locator('.device-facts').innerText().then(t=>t.includes('TEST-DEVICE-001')&&t.includes('0.1.2')),'device information comes from status response');
 await page.screenshot({path:'out/design-preview/home-small-healthy.png'});
 await page.evaluate(()=>{window.ocm.getStatus=async()=>({ok:true,data:{connected:false,abilityAlive:false}})});
 await page.getByRole('button',{name:'全面体检',exact:true}).click();
 await page.getByRole('heading',{name:'部分服务需要关注',exact:true}).waitFor();
 assert(await page.locator('.check-result.partial').count()===2,'checkup identifies disconnected channel and unavailable ability');
 await page.setViewportSize({width:980,height:660});
 await page.evaluate(()=>{window.ocm.getStatus=async()=>{throw Error('核心服务未运行')}});
 await page.getByRole('button',{name:'全面体检',exact:true}).click();
 await page.getByRole('heading',{name:'暂时无法读取设备状态',exact:true}).waitFor();
 assert(await page.locator('.check-result.loading').count()===2,'offline dependencies stay pending instead of showing healthy');
 await page.waitForTimeout(3500);
 await page.mouse.move(970,650);
 await page.screenshot({path:'out/design-preview/home-light.png'});
 assert(await page.locator('.rail-nav').evaluate(el=>el.getBoundingClientRect().bottom>500),'navigation spreads into available vertical space');
 await page.getByRole('button',{name:'病毒查杀',exact:false}).click();
 await page.getByRole('heading',{name:'病毒查杀',exact:true}).waitFor();
 assert(true,'home shortcut opens the corresponding feature');
 if(errors.length)throw Error(errors.join('\n'));
 }finally{await browser.close();server.close();}
})().catch(e=>{console.error(e);server.close();process.exitCode=1;});
