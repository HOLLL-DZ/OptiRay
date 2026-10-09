const { chromium } = require('C:/Users/Roman/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
(async()=>{
 const browser=await chromium.launch({channel:'msedge',headless:true});
 const page=await browser.newPage({viewport:{width:1280,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('http://127.0.0.1:13884',{waitUntil:'networkidle'});
 await page.screenshot({path:'D:/Chat__GPT/OptiRay/go-optiray/build/ui-check/desktop.png',fullPage:true});
 console.log(JSON.stringify({cards:await page.locator('.server-card').count(),groups:await page.locator('.sub-section').count(),errors,unreplaced:await page.locator('body').innerText().then(t=>/__[A-Z][A-Z0-9_]*__/.test(t)),overflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)}));
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:'D:/Chat__GPT/OptiRay/go-optiray/build/ui-check/mobile.png',fullPage:true});
 console.log(JSON.stringify({mobileOverflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)}));
 await browser.close();if(errors.length)process.exitCode=1;
})().catch(e=>{console.error(e);process.exit(1)});

