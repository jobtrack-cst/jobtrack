// Optional local check: Node 22+ and an already-installed Chrome, no npm packages.
// Uses a temporary Chrome profile with its native default page zoom set to 200%.
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');

const chromePath = [process.env.CHROME_BINARY, '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/usr/bin/google-chrome', '/usr/bin/chromium'].find(file => file && fs.existsSync(file));
if (!chromePath) throw new Error('Installed Chrome not found. Set CHROME_BINARY; no browser is installed by this check.');
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'jobtrack-native-zoom-'));
fs.mkdirSync(path.join(profile, 'Default'));
// Chromium uses partition key x for the default profile partition.
// https://chromium.googlesource.com/chromium/src/+/lkgr/chrome/browser/ui/zoom/chrome_zoom_level_prefs.cc
fs.writeFileSync(path.join(profile, 'Default', 'Preferences'), JSON.stringify({ partition: { default_zoom_level: { x: Math.log(2) / Math.log(1.2) } } }));
const chrome = spawn(chromePath, ['--headless=new', '--no-first-run', '--no-default-browser-check', '--remote-debugging-port=0',
  `--user-data-dir=${profile}`, '--window-size=1280,960', 'about:blank'], { stdio: 'ignore' });
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
let socket;
let send;
let currentRole = 'applicant';

(async () => {
  try {
    const portFile = path.join(profile, 'DevToolsActivePort');
    for (let attempt = 0; !fs.existsSync(portFile) && attempt < 150; attempt++) await pause(100);
    assert(fs.existsSync(portFile), 'Chrome must expose its temporary debugging endpoint');
    const port = fs.readFileSync(portFile, 'utf8').split('\n')[0];
    const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
    socket = new WebSocket(targets.find(target => target.type === 'page').webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, { once: true }); socket.addEventListener('error', reject, { once: true }); });
    let sequence = 0;
    const pending = new Map();
    send = (method, params = {}) => new Promise((resolve, reject) => {
      const id = ++sequence;
      const timer = setTimeout(() => { pending.delete(id); reject(new Error(`Timed out: ${method}`)); }, 10000);
      pending.set(id, { resolve, reject, timer });
      socket.send(JSON.stringify({ id, method, params }));
    });
    socket.addEventListener('message', async message => {
      const data = JSON.parse(String(message.data));
      if (data.id && pending.has(data.id)) {
        const task = pending.get(data.id); pending.delete(data.id); clearTimeout(task.timer);
        if (data.error) task.reject(new Error(data.error.message)); else task.resolve(data.result);
      } else if (data.method === 'Fetch.requestPaused') {
        const { requestId, request } = data.params;
        const user = { userId: '11111111-1111-4111-8111-111111111111', role: `ROLE_${currentRole.toUpperCase()}`, fullName: 'Synthetic Zoom User', email: 'zoom@example.test' };
        const body = request.url.includes('/auth/me') ? { user } : {};
        await send('Fetch.fulfillRequest', { requestId, responseCode: request.method === 'OPTIONS' ? 204 : 200,
          responseHeaders: [{ name: 'Content-Type', value: 'application/json' }, { name: 'Access-Control-Allow-Origin', value: '*' },
            { name: 'Access-Control-Allow-Headers', value: 'authorization,content-type' }, { name: 'Access-Control-Allow-Methods', value: 'GET,POST,PUT,DELETE,OPTIONS' }],
          body: request.method === 'OPTIONS' ? '' : Buffer.from(JSON.stringify(body)).toString('base64') });
      }
    });
    await send('Page.enable');
    const version = await send('Browser.getVersion');
    console.log(`Native zoom browser: ${version.product}`);
    await send('Fetch.enable', { patterns: [{ urlPattern: 'http://localhost:8000/v1/*', requestStage: 'Request' }] });
    await send('Page.addScriptToEvaluateOnNewDocument', { source: `sessionStorage.setItem('jobtrack.auth', JSON.stringify({ accessToken: 'jt101-native-zoom', expiresAt: Date.now() + 3600000 }));` });
    const evaluate = async expression => {
      const response = await send('Runtime.evaluate', { expression, returnByValue: true });
      if (response.exceptionDetails) throw new Error(response.exceptionDetails.text);
      return response.result.value;
    };
    for (const role of ['applicant', 'employer', 'admin']) {
      currentRole = role;
      await send('Page.navigate', { url: `http://localhost:5173/${role}` });
      let ready = false;
      for (let attempt = 0; !ready && attempt < 100; attempt++) {
        ready = await evaluate(`!!document.querySelector('.workspace-content h1') && document.querySelector('.workspace-content h1').textContent.toLowerCase().startsWith('${role}')`);
        if (!ready) await pause(100);
      }
      assert(ready, `${role} authorized home must render`);
      const metrics = await evaluate(`({ width: innerWidth, outer: outerWidth, client: document.documentElement.clientWidth, dpr: devicePixelRatio, scroll: document.documentElement.scrollWidth, scale: visualViewport.scale })`);
      assert(Math.abs(metrics.outer / metrics.width - 2) < .08, `Native 200% zoom must halve CSS viewport: ${JSON.stringify(metrics)}`);
      assert(metrics.dpr >= 1.9, 'Native zoom must increase the device pixel ratio');
      assert(metrics.scroll <= metrics.client, `${role} must reflow without horizontal overflow`);
      await evaluate(`document.querySelector('#account-menu-button').focus()`);
      await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, text: '\r', unmodifiedText: '\r' });
      await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
      await pause(250);
      const account = await evaluate(`(() => {
        const menu = document.querySelector('#account-menu [role=menu]');
        const rect = menu.closest('.MuiPaper-root').getBoundingClientRect();
        return { left: rect.left, right: rect.right, width: innerWidth, count: menu.querySelectorAll('[role=menuitem]').length,
          logout: menu.querySelector('button')?.textContent, scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth };
      })()`);
      assert(account.left >= 0 && account.right <= account.width, 'Account menu must fit native zoom viewport');
      assert.equal(account.count, role === 'admin' ? 1 : 2, 'Only authorized account actions appear');
      assert.equal(account.logout, 'Log out', 'Logout remains discoverable at native zoom');
      assert(account.scroll <= account.client, 'Open account menu must not cause horizontal overflow');
      await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
      await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
      await pause(200);
      assert(await evaluate(`!document.querySelector('#account-menu') && document.activeElement.id === 'account-menu-button'`), 'Escape closes account menu and restores trigger focus');
      assert(await evaluate(`!!document.querySelector('button[aria-label="Open navigation"]')`), 'Native zoom must select mobile navigation');
      await evaluate(`document.querySelector('button[aria-label="Open navigation"]').click()`);
      await pause(250);
      assert(await evaluate(`document.querySelector('[role=dialog]').getBoundingClientRect().width <= innerWidth`), 'Drawer must fit zoomed viewport');
      await evaluate(`document.querySelector('button[aria-label="Close navigation"]').click()`);
      await pause(250);
      await evaluate(`document.querySelector('button[aria-label="Open navigation"]').click()`);
      await pause(250);
      assert(await evaluate(`document.querySelector('button[aria-label="Open navigation"]').getAttribute('aria-expanded') === 'true'`), 'Drawer must reopen after dismissal');
      await evaluate(`document.querySelector('button[aria-label="Close navigation"]').click()`);
      await pause(250);
      const destination = role === 'admin' ? 'analytics' : 'jobs';
      await evaluate(`document.querySelector('.workspace-content a[href="/${role}/${destination}"]').click()`);
      await pause(250);
      assert.equal(await evaluate('location.pathname'), `/${role}/${destination}`);
      assert(await evaluate('document.documentElement.scrollWidth <= document.documentElement.clientWidth'), 'Feature page must reflow at native zoom');
      console.log(`PASS ${role}: native 200% zoom, ${metrics.outer}px window / ${metrics.width}px CSS viewport, DPR ${metrics.dpr}; account menu/actions/Escape/focus, home, drawer and ${destination} route`);
    }
  } finally {
    if (send && socket?.readyState === WebSocket.OPEN) await send('Browser.close').catch(() => {});
    socket?.close();
    if (chrome.exitCode === null) chrome.kill();
    await pause(250);
    fs.rmSync(profile, { recursive: true, force: true });
  }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
