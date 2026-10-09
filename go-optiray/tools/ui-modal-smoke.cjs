const { chromium } = require('C:/Users/Roman/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
(async()=>{
 const browser=await chromium.launch({channel:'msedge',headless:true});
 const page=await browser.newPage({viewport:{width:1280,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('http://127.0.0.1:13884',{waitUntil:'networkidle'});
 await page.getByRole('button',{name:'Настройки',exact:true}).click();
 const selected=await page.locator('#subscription_client_profile').inputValue();
 await page.screenshot({path:'D:/Chat__GPT/OptiRay/go-optiray/build/ui-check/settings.png',fullPage:true});
 await page.keyboard.press('Escape');await page.getByRole('link',{name:'Маршрутизация',exact:true}).click();
 await page.waitForTimeout(350);await page.screenshot({path:'D:/Chat__GPT/OptiRay/go-optiray/build/ui-check/routing.png',fullPage:true});
 console.log(JSON.stringify({selected,errors,dialogs:await page.getByRole('dialog').count(),desktopOverflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)}));
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:'D:/Chat__GPT/OptiRay/go-optiray/build/ui-check/mobile-routing.png',fullPage:true});
 console.log(JSON.stringify({mobileOverflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)}));
 await browser.close();if(errors.length||selected!=='happ')process.exitCode=1;
})().catch(e=>{console.error(e);process.exit(1)});
