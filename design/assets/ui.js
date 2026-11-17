/* 共用控件工厂。只接受文字数据；SVG 均来自本地受控字符串。 */
(() => {
  const icons = {
    plus: '<path d="M12 5v14M5 12h14"/>',
    settings: '<circle cx="12" cy="12" r="3"/><path d="M12 2.75v2.1m0 14.3v2.1M2.75 12h2.1m14.3 0h2.1M5.46 5.46l1.49 1.49m10.1 10.1 1.49 1.49m0-13.08-1.49 1.49M6.95 17.05l-1.49 1.49"/><circle cx="12" cy="12" r="7.15"/>',
    close: '<path d="m6 6 12 12M6 18 18 6"/>',
    menu: '<path d="M5 7h14M5 12h14M5 17h14"/>',
    arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
    check: '<path d="m5 12 4 4 10-10"/>'
  };
  function el(tag, attrs = {}, children = []) {
    const node = document.createElement(tag);
    for (const [key, value] of Object.entries(attrs)) {
      if (value == null || value === false) continue;
      if (key === 'text') node.textContent = value;
      else if (key === 'class') node.className = value;
      else if (key.startsWith('on')) node.addEventListener(key.slice(2).toLowerCase(), value);
      else if (key === 'value' || key === 'checked' || key === 'disabled') node[key] = value;
      else node.setAttribute(key, value === true ? '' : String(value));
    }
    for (const child of children) if (child != null) node.append(child);
    return node;
  }
  function icon(name) {
    const template = document.createElement('template');
    template.innerHTML = `<svg class="gf-icon" viewBox="0 0 24 24" aria-hidden="true">${icons[name] || icons.menu}</svg>`;
    return template.content.firstElementChild;
  }
  function button(label, attrs = {}) {
    return el('button', { type: 'button', class: 'gf-button', text: label, ...attrs });
  }
  function iconButton(name, label, handler, className = '') {
    return el('button', { type: 'button', class: `gf-icon-button ${className}`, 'aria-label': label, title: label, onClick: handler }, [icon(name)]);
  }
  function input(attrs = {}) { return el('input', { class: 'gf-input', ...attrs }); }
  function tab(item, active, handlers = {}) {
    const label = button(item.title, { class: 'gf-tab-label', role: 'tab', 'aria-selected': String(active), title: item.title, onClick: handlers.select || (() => {}) });
    const row = el('div', { class: `gf-tab${active ? ' is-active' : ''}`, role: 'presentation' }, [label]);
    if (active) row.append(iconButton('close', `关闭标签页 ${item.title}`, handlers.close || (() => {}), 'gf-tab-close'));
    return row;
  }
  function authForm(prompt, handlers = {}, prefix = 'auth') {
    const form = el('form', { class: 'gf-auth-form' });
    form.append(el('h2', { text: '网站需要身份验证' }), el('p', { text: `主机：${prompt.host || 'protected.example.com'}` }), el('p', { text: '认证范围：Private workspace' }));
    const username = input({ id: `${prefix}-username`, name: 'username', placeholder: '用户名', 'aria-label': '用户名', value: prompt.username || '', disabled: prompt.busy, autocomplete: 'off' });
    const password = input({ id: `${prefix}-password`, name: 'password', type: 'password', placeholder: '密码', 'aria-label': '密码', value: prompt.password || '', disabled: prompt.busy, autocomplete: 'off' });
    username.addEventListener('input', () => { prompt.username = username.value; });
    password.addEventListener('input', () => { prompt.password = password.value; });
    form.append(username,password);
    const message = el('p', { class: prompt.error ? 'gf-error' : 'gf-helper', role: 'status', text: prompt.error || (prompt.busy ? '正在验证…' : '') });
    form.append(message,el('div', { class: 'gf-footer' }, [button('取消', { onClick: handlers.cancel || (() => {}) }),button('登录', { type: 'submit', class: 'gf-button primary', disabled: prompt.busy })]));
    form.addEventListener('submit', event => { event.preventDefault(); if (handlers.submit) handlers.submit(prompt); });
    return form;
  }
  const defaultKeys = [
    { key: '-', meta: true, ctrl: false, alt: false, shift: true },
    { key: '=', meta: true, ctrl: false, alt: false, shift: true },
    { key: 'T', meta: true, ctrl: false, alt: false, shift: true },
    { key: 'H', meta: true, ctrl: false, alt: false, shift: true }
  ];
  const labels = ['降低不透明度','提高不透明度','切换穿透','隐藏 / 显示'];
  function displayKey(binding) { return [binding.ctrl && '⌃',binding.alt && '⌥',binding.shift && '⇧',binding.meta && '⌘',binding.key].filter(Boolean).join(' '); }
  function defaultSettings() { return { min: 0, max: 100, drag: { meta:true, ctrl:false, alt:false, shift:false }, keys: defaultKeys.map(item => ({ ...item })) }; }
  window.GF = { el, icon, button, iconButton, input, tab, authForm, displayKey, defaultSettings, labels };
  const themeSelect = document.querySelector('#theme');
  function setTheme(value) {
    const dark = value === 'dark' || (value === 'system' && matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
    document.dispatchEvent(new Event('themechange'));
  }
  if (themeSelect) {
    themeSelect.addEventListener('change', () => setTheme(themeSelect.value));
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if (themeSelect.value === 'system') setTheme('system'); });
    setTheme(themeSelect.value);
  }
})();
