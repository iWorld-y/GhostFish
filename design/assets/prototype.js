/* 原型交互仅在当前网页内运行，不访问外部网站或系统设置。 */
(() => {
  const { el, button, iconButton, input, tab, authForm, displayKey, defaultSettings, labels } = GF;
  const $ = selector => document.querySelector(selector);
  const scenes = {
    browse: { title: '浮动浏览器', description: '标签位于顶部，网页提交后隐藏地址栏。保持内容优先的紧凑布局。', rules: ['当前标签显示关闭按钮，长标题单行截断。','关闭最后一个标签会退出应用，原型显示退出状态。'], source: 'internal/browser/browser_ui_mac.go · tabs.go · browser.go' },
    blank: { title: '新标签页', description: '新建标签后聚焦地址栏，输入网址并按 Enter 开始导航。', rules: ['未填写协议时补全 https://，仅接受 HTTP 和 HTTPS。','网页显示后隐藏地址栏，不增加搜索框或导航工具栏。'], source: 'internal/browser/target.go · tabs.go · browser_ui.go' },
    settings: { title: '窗口设置', description: '编辑草稿，保存后生效。取消、关闭或按 Escape 均丢弃草稿。', rules: ['录入快捷键期间，Escape 只退出录入。','设置打开时暂时暂停穿透，关闭后恢复生效配置。'], source: 'internal/settings/view.go · controller.go · internal/desktop/settings.go' },
    auth: { title: '网站身份验证', description: '当前关联源码使用独立认证窗口，窗口不透明，并随活动标签显示或隐藏。', rules: ['切换标签保留输入，关闭标签取消该标签的认证。','验证期间禁用输入与登录，失败后保留输入以便重试。'], source: 'internal/browser/auth_ui.go · auth_controller.go' },
    failure: { title: '首次导航失败', description: '第一次加载失败时恢复地址输入，保留目标网址和错误信息。', rules: ['修改输入会清除原错误，按 Enter 可重试。','已打开网页的后续加载失败，不重新显示地址栏。'], source: 'internal/browser/tabs.go → failPageLoad · browser.go' },
    through: { title: '鼠标穿透', description: '穿透时窗口内容不接收点击，使用画布外的托盘入口恢复操作。', rules: ['整窗不透明度默认 85%，每次调整 10 个百分点。','设置打开时保持可点；原型不改变操作系统的鼠标行为。'], source: 'internal/desktop/actions.go · recovery.go · internal/config/state.go' },
    hidden: { title: '隐藏与恢复', description: '隐藏保留标签和内容，托盘菜单会从“隐藏窗口”变为“显示窗口”。', rules: ['隐藏与退出不同，退出会关闭全部标签。','原型始终保留画布外的恢复入口。'], source: 'internal/desktop/tray.go · actions.go' }
  };
  let nextId = 1;
  const state = { tabs: [], active: null, settings: defaultSettings(), opacity:85, through:false, hidden:false, quit:false, scene:'browse', draft:null, recording:-1, authOutcome:'error', generation:0 };
  const dialog = $('#settings-dialog');
  function freshTab(title = '新标签页', loaded = false) { return { id: nextId++, title, loaded, input:'', error:'', url: loaded ? 'https://example.com/notes' : '', auth:null }; }
  function activeTab() { return state.tabs.find(item => item.id === state.active); }
  function announce(text) { $('#notice').textContent = text; $('#terminal-line').textContent = `ghostfish: ${text}`; }
  function sceneInfo(name) {
    const info = scenes[name];
    $('#scene-title').textContent = info.title;
    $('#scene-description').textContent = info.description;
    $('#behavior-rules').replaceChildren(...info.rules.map(text => el('li',{text})));
    $('#source-text').textContent = info.source;
    document.querySelectorAll('[data-scene]').forEach(item => item.setAttribute('aria-current',String(item.dataset.scene === name)));
    $('#auth-options').hidden = name !== 'auth';
  }
  function loadScene(name) {
    if (dialog.open) dialog.close();
    state.generation++;
    state.scene = name; state.through = false; state.hidden = false; state.quit = false;
    state.opacity = Math.max(state.settings.min,Math.min(state.settings.max,85));
    const initial = freshTab(name === 'blank' || name === 'failure' || name === 'auth' ? '新标签页' : '工作笔记', !['blank','failure','auth'].includes(name));
    state.tabs = [initial]; state.active = initial.id;
    if (name === 'browse') state.tabs.push(freshTab('新标签页'));
    if (name === 'failure') { initial.input = 'https://unavailable.example.com'; initial.error = '载入网址失败：无法连接服务器'; }
    if (name === 'auth') { initial.input = 'https://protected.example.com'; initial.auth = { host:'protected.example.com', username:'', password:'', error:'',busy:false }; state.tabs.push(freshTab('工作笔记',true)); }
    if (name === 'through') state.through = true;
    if (name === 'hidden') state.hidden = true;
    sceneInfo(name); render();
    if (name === 'settings') openSettings();
    if (name === 'blank') $('#address-input')?.focus();
    announce(`已切换到${scenes[name].title}演示`);
  }
  function addTab() {
    if (state.quit) { state.quit = false; state.hidden = false; }
    const item = freshTab(); state.tabs.push(item); state.active = item.id; render(); $('#address-input')?.focus();
  }
  function closeTab(item) {
    const index = state.tabs.indexOf(item); state.tabs.splice(index,1);
    if (!state.tabs.length) { state.quit = true; announce('最后一个标签已关闭，应用退出（模拟）'); }
    else if (state.active === item.id) state.active = state.tabs[Math.min(index,state.tabs.length-1)].id;
    render();
  }
  function renderTabs() {
    const target = $('#tab-list');
    target.replaceChildren(...state.tabs.map(item => tab(item,item.id === state.active,{ select:() => { state.active = item.id; render(); },close:() => closeTab(item) })));
  }
  function normalizeURL(value) {
    const raw = value.trim();
    if (!raw) throw new Error('目标网址为空');
    if (/\s/.test(raw)) throw new Error('请输入有效网址，地址栏不支持关键词搜索');
    if (/^(mailto|tel|data|javascript|file|ftp|ws|wss):/i.test(raw)) throw new Error('只支持 http 与 https');
    const url = new URL(raw.includes('://') ? raw : `https://${raw}`);
    if (!['http:','https:'].includes(url.protocol)) throw new Error('只支持 http 与 https');
    return url.href;
  }
  function navigate(item) {
    try { item.url = normalizeURL(item.input); }
    catch(error) { item.error = error.message.startsWith('Invalid') ? '网址无法解析，请检查输入' : error.message; renderContent(); $('#address-input')?.focus(); return; }
    item.error = ''; item.auth = null; item.loaded = false; item.busy = true;
    render();
    const generation = state.generation;
    setTimeout(() => {
      if (generation !== state.generation || !state.tabs.includes(item)) return;
      item.busy = false;
      if (new URL(item.url).hostname === 'unavailable.example.com') item.error = '载入网址失败：无法连接服务器';
      else if (new URL(item.url).hostname === 'protected.example.com') item.auth = { host:'protected.example.com', username:'',password:'',error:'',busy:false };
      else { item.loaded = true; item.title = '工作笔记'; }
      render(); announce('导航演示已更新，未连接真实网站');
    },650);
  }
  function examplePage(item) {
    const page = el('article',{class:'example-page'});
    page.append(el('div',{class:'example-header'},[el('span',{text:'工作笔记'}),el('span',{text:'本地示例网页',class:'toolbar-label'})]));
    page.append(el('p',{class:'eyebrow',text:'WORKSPACE / NOTES'}),el('h2',{text:'留一点空间，专注当前的事。'}),el('p',{text:'这里是一份中性的网页示例，用来观察 GhostFish 的窗口、标签和透明度。页面内容不属于应用自身的设计规范。'}));
    const list = el('div',{class:'example-list'});
    ['整理今天的工作记录','查看待处理事项','记录一个新的想法'].forEach((text,index) => list.append(el('div',{class:'example-row'},[el('span',{class:'index',text:`0${index+1}`}),el('span',{class:'title',text}),el('small',{text:'示例内容'})])));
    page.append(list,el('p',{text:`当前演示地址：${item.url || 'https://example.com/notes'}`}));
    return page;
  }
  function renderContent() {
    const item = activeTab();
    $('#address-area').replaceChildren(); $('#web-content').replaceChildren();
    if (!item || state.quit) return;
    if (!item.loaded) {
      const field = input({id:'address-input',value:item.input,placeholder:'输入网址','aria-label':'地址栏',autocomplete:'off',spellcheck:'false', 'aria-describedby':item.error ? 'address-error' : null});
      field.addEventListener('input',() => { item.input = field.value; item.error=''; $('#address-error')?.remove(); field.removeAttribute('aria-describedby'); });
      const form = el('form',{class:'gf-address'},[field]);
      form.addEventListener('submit',event => { event.preventDefault(); navigate(item); });
      if (item.error) form.append(el('p',{class:'gf-error',id:'address-error',role:'alert',text:item.error}));
      $('#address-area').append(form);
    }
    if (item.loaded) $('#web-content').append(examplePage(item));
    else if (item.busy) $('#web-content').append(el('div',{class:'blank-state',role:'status'},[el('p',{text:'正在加载本地演示…'})]));
  }
  function submitAuth(prompt) {
    const item = activeTab();
    if (!prompt.username.trim()) { prompt.error = '请输入用户名。'; renderAuth(); return; }
    prompt.busy = true; prompt.error = ''; renderAuth();
    const outcome = state.authOutcome;
    setTimeout(() => {
      if (!state.tabs.includes(item) || item.auth !== prompt) return;
      prompt.busy = false;
      if (outcome === 'success') { item.auth=null; item.loaded=true; item.title='工作笔记'; item.url='https://protected.example.com'; announce('验证成功状态演示完成，未提交账号或密码'); }
      else prompt.error='账号或密码不正确，请重试。';
      render();
    },800);
  }
  function renderAuth() {
    const item = activeTab(); const target = $('#auth-window');
    const shown = !!item?.auth && !state.quit;
    target.hidden = !shown; $('#scene-windows').classList.toggle('has-auth',shown);
    target.replaceChildren();
    if (!shown) return;
    target.append(el('div',{class:'gf-auth-caption',text:'网站需要身份验证'}),authForm(item.auth,{submit:submitAuth,cancel:() => { item.auth=null; item.error='身份验证已取消（演示）'; render(); }}));
  }
  function renderStatus() {
    const through = state.through && !dialog.open;
    $('#window-frame').style.setProperty('--preview-opacity',state.opacity/100);
    $('#window-frame').classList.toggle('is-through',through);
    $('#window-frame').inert = through || state.hidden || state.quit;
    $('#opacity-value').textContent = `${state.opacity}%`;
    $('#through-value').textContent = state.through ? (dialog.open ? '设置中暂停' : '已开启') : '已关闭';
    $('#visible-value').textContent = state.quit ? '已退出' : state.hidden ? '已隐藏' : '显示中';
    $('#tab-count').textContent = String(state.tabs.length);
    $('#opacity-range').value = state.opacity;
    $('#opacity-range').min = state.settings.min; $('#opacity-range').max=state.settings.max;
    $('#through-toggle').setAttribute('aria-checked',String(state.through));
    $('#through-check').textContent = state.through ? '✓' : '';
    $('#through-subtitle').textContent = state.through && dialog.open ? '设置打开时暂时暂停' : '';
    $('#visibility-label').textContent = state.hidden ? '显示窗口' : '隐藏窗口';
    $('#window-frame').hidden = state.hidden || state.quit;
    $('#hidden-placeholder').hidden = !state.hidden && !state.quit;
    $('#hidden-title').textContent = state.quit ? '应用已退出' : '窗口已隐藏';
    $('#hidden-description').textContent = state.quit ? '最后一个标签或退出操作会结束应用。可以重新开始演示。' : '标签与网页仍然保留，可通过托盘恢复。';
    $('#restore-window').textContent = state.quit ? '重新开始演示' : '显示窗口';
    $('#open-settings').disabled = state.quit;
  }
  function render() { renderTabs(); renderContent(); renderAuth(); renderStatus(); }
  function setOpacity(value) { state.opacity=Math.max(state.settings.min,Math.min(state.settings.max,Number(value))); renderStatus(); announce(`不透明度 ${state.opacity}%`); }
  function toggleThrough() { state.through=!state.through; renderStatus(); announce(`穿透 ${state.through ? '开' : '关'}（网页内模拟）`); }
  function toggleHidden() { if(state.quit)return; state.hidden=!state.hidden; renderStatus(); announce(state.hidden?'窗口已隐藏（模拟）':'窗口已恢复（模拟）'); }
  function quit() { state.quit=true; state.tabs=[]; render(); closeTray(); announce('应用已退出（模拟）'); }
  function closeTray() { $('#tray-menu').hidden=true; $('#tray-trigger').setAttribute('aria-expanded','false'); }
  function renderShortcuts() {
    $('#shortcut-list').replaceChildren(...state.draft.keys.map((binding,index) => {
      const keyButton=button(state.recording===index?'请按快捷键':displayKey(binding),{class:`gf-button gf-shortcut${state.recording===index?' is-recording':''}`,'aria-label':`${labels[index]}快捷键`,onClick:()=> {state.recording=index;renderShortcuts();$('#shortcut-list').children[index].querySelector('button').focus();}});
      return el('div',{class:'gf-shortcut-row'},[el('span',{text:labels[index]}),keyButton]);
    }));
  }
  function openSettings() {
    if(state.quit)return;
    state.draft=JSON.parse(JSON.stringify(state.settings)); state.recording=-1;
    $('#min-opacity').value=state.draft.min; $('#max-opacity').value=state.draft.max; $('#settings-error').textContent='';
    document.querySelectorAll('[data-modifier]').forEach(node=>{node.checked=state.draft.drag[node.dataset.modifier];});
    renderShortcuts();
    if(!dialog.open)dialog.showModal();
    renderStatus(); $('#min-opacity').focus();
  }
  function settingsError(text) { $('#settings-error').textContent=text; }
  function saveSettings(event) {
    event.preventDefault();
    if(state.recording>=0)return settingsError('请先完成或取消快捷键录入');
    const low=$('#min-opacity').value.trim(),high=$('#max-opacity').value.trim();
    if(!/^\d+$/.test(low)||!/^\d+$/.test(high)||Number(low)>Number(high)||Number(high)>100)return settingsError('请输入 0% 至 100% 的整数，且下限不能大于上限');
    const next=state.draft;next.min=Number(low);next.max=Number(high);
    document.querySelectorAll('[data-modifier]').forEach(node=>{next.drag[node.dataset.modifier]=node.checked;});
    if(!Object.values(next.drag).some(Boolean))return settingsError('窗口拖动：请至少选择一个修饰键');
    if(new Set(next.keys.map(displayKey)).size!==4)return settingsError('快捷键不能重复');
    state.settings=JSON.parse(JSON.stringify(next)); state.opacity=Math.max(next.min,Math.min(next.max,state.opacity));
    dialog.close();renderStatus();announce('设置已在当前演示中生效，未修改系统配置');
  }
  function physicalKey(event) {
    if(/^Key[A-Z]$/.test(event.code))return event.code.slice(3);
    if(/^Digit[0-9]$/.test(event.code))return event.code.slice(5);
    return ({Minus:'-',Equal:'=',Space:'Space'})[event.code] || (/^F([1-9]|1[0-2])$/.test(event.code)?event.code:null);
  }
  document.addEventListener('keydown',event=>{
    if(dialog.open && state.recording>=0) {
      event.preventDefault();event.stopPropagation();
      if(event.key==='Escape'){state.recording=-1;renderShortcuts();return;}
      if(['Meta','Control','Alt','Shift'].includes(event.key))return;
      const key=physicalKey(event);
      if(!key||!(event.metaKey||event.ctrlKey||event.altKey))return settingsError('请选择受支持的主键和 ⌘、Control 或 Option 修饰键');
      state.draft.keys[state.recording]={key,meta:event.metaKey,ctrl:event.ctrlKey,alt:event.altKey,shift:event.shiftKey};
      state.recording=-1;settingsError('');renderShortcuts();return;
    }
    if(event.key==='Escape')closeTray();
  },true);
  dialog.addEventListener('cancel',event=>{if(state.recording>=0){event.preventDefault();state.recording=-1;renderShortcuts();}});
  dialog.addEventListener('close',()=>{state.recording=-1;state.draft=null;renderStatus();});
  dialog.addEventListener('click',event=>{const rect=dialog.getBoundingClientRect();if(event.target===dialog&&(event.clientX<rect.left||event.clientX>rect.right||event.clientY<rect.top||event.clientY>rect.bottom))dialog.close();});
  $('#settings-form').addEventListener('submit',saveSettings);
  document.querySelectorAll('[data-close-settings]').forEach(node=>node.addEventListener('click',()=>dialog.close()));
  document.querySelectorAll('[data-scene]').forEach(node=>node.addEventListener('click',()=>loadScene(node.dataset.scene)));
  $('#add-tab').append(GF.icon('plus'));$('#add-tab').addEventListener('click',addTab);
  $('#open-settings').append(GF.icon('settings'));$('#open-settings').addEventListener('click',openSettings);
  $('#reset').addEventListener('click',()=>{state.settings=defaultSettings();loadScene('browse');});
  $('#opacity-range').addEventListener('input',event=>setOpacity(event.target.value));
  $('#opacity-down').addEventListener('click',()=>setOpacity(state.opacity-10));$('#opacity-up').addEventListener('click',()=>setOpacity(state.opacity+10));
  $('#tray-trigger').addEventListener('click',()=>{const menu=$('#tray-menu');menu.hidden=!menu.hidden;$('#tray-trigger').setAttribute('aria-expanded',String(!menu.hidden));if(!menu.hidden)menu.querySelector('button').focus();});
  $('#tray-menu').addEventListener('keydown',event=>{const items=[...$('#tray-menu').querySelectorAll('button')];const index=items.indexOf(document.activeElement);if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();items[(index+(event.key==='ArrowDown'?1:items.length-1))%items.length].focus();}if(event.key==='Escape'){closeTray();$('#tray-trigger').focus();}});
  document.addEventListener('click',event=>{if(!event.target.closest('.tray-wrapper'))closeTray();});
  $('#through-toggle').addEventListener('click',()=>{toggleThrough();closeTray();});$('#visibility-toggle').addEventListener('click',()=>{toggleHidden();closeTray();});$('#quit').addEventListener('click',quit);
  $('#window-close').addEventListener('click',quit);$('#window-hide').addEventListener('click',toggleHidden);
  $('#window-grow').addEventListener('click',()=>{$('#window-frame').classList.toggle('expanded');$('#window-frame').style.height=$('#window-frame').classList.contains('expanded')?'800px':'';announce('窗口尺寸已切换（模拟）');});
  $('#restore-window').addEventListener('click',()=>{if(state.quit)loadScene('browse');else{state.hidden=false;render();}});
  $('#auth-result').addEventListener('change',event=>{state.authOutcome=event.target.value;});
  loadScene('browse');
})();
