/* 组件目录使用与主原型相同的控件工厂及设计变量。 */
(() => {
  const {el,button,iconButton,input,tab,authForm,displayKey,defaultSettings,labels}=GF;
  const $=selector=>document.querySelector(selector);
  function feedback(message) { $('#component-feedback').textContent=message; }
  function showColors() {
    const styles=getComputedStyle(document.documentElement);
    const colors=[['窗口背景','--app-bg','源码显式值'],['内容表面','--surface','网页映射'],['悬停表面','--surface-hover','网页映射'],['分隔与边框','--border','网页映射'],['主要文字','--text','网页映射'],['次要文字','--muted','网页映射'],['主要操作','--accent','网页映射'],['错误提示','--danger','网页映射']];
    $('#color-grid').replaceChildren(...colors.map(([name,token,source])=>el('div',{class:'swatch'},[el('div',{class:'swatch-color',style:`background:var(${token})`}),el('div',{class:'swatch-info'},[el('span',{text:name}),el('code',{text:`${token}  ${styles.getPropertyValue(token).trim()}`}),el('small',{text:source})])])));
  }
  $('#button-examples').append(button('取消',{onClick:()=>feedback('已点击次要按钮')}),button('保存',{class:'gf-button primary',onClick:()=>feedback('已点击主要按钮，未保存真实配置')}),button('不可用',{disabled:true}),button('悬停状态',{class:'gf-button is-hover',onClick:()=>feedback('固定展示悬停样式')}));
  const loading=button('登录',{class:'gf-button primary',onClick:()=>{loading.disabled=true;loading.textContent='正在验证…';setTimeout(()=>{loading.disabled=false;loading.textContent='登录';feedback('加载状态演示已结束');},1000);}});
  $('#button-examples').append(loading);
  $('#icon-examples').append(iconButton('plus','新建标签页',()=>feedback('新建标签页按钮')),iconButton('settings','设置',()=>feedback('设置按钮')),iconButton('close','关闭标签页',()=>feedback('关闭标签页按钮')));
  const fields=[['常规','网址',{placeholder:'输入网址'}],['有内容','网址内容',{value:'https://example.com'}],['错误','错误网址',{value:'https://unavailable.example.com','aria-invalid':'true','aria-describedby':'field-error'}],['禁用','验证中的用户名',{value:'demo',disabled:true}]];
  fields.forEach(([title,name,attrs],index)=>$('#input-examples').append(el('label',{class:'gf-field',text:title},[input({id:`sample-input-${index}`,'aria-label':name,...attrs})])));
  $('#input-examples').append(el('p',{class:'gf-error',id:'field-error',text:'载入网址失败：无法连接服务器'}));
  let active=0;
  const tabs=[{title:'工作笔记'},{title:'一段较长的网页标题，用于展示单行截断'},{title:'新标签页'}];
  function drawTabs() {
    $('#tab-examples').replaceChildren(...tabs.map((item,index)=>tab(item,index===active,{select:()=>{active=index;drawTabs();feedback(`已选中 ${item.title}`);},close:()=>feedback('组件样例保留标签；完整关闭流程见交互原型')})),iconButton('plus','新建标签页',()=>feedback('完整新建流程见交互原型'),'gf-plus'));
  }
  drawTabs();
  const keys=defaultSettings().keys;
  $('#shortcut-examples').append(...keys.map((binding,index)=>el('div',{class:'gf-shortcut-row'},[el('span',{text:labels[index]}),button(displayKey(binding),{class:'gf-button gf-shortcut',onClick:()=>feedback('请在交互原型的设置弹层中录入快捷键')})])));
  $('#shortcut-recording').append(button('请按快捷键',{class:'gf-button gf-shortcut is-recording',onClick:()=>feedback('录入态样例，按键录入在设置原型中演示')}));
  const errorPrompt={host:'protected.example.com',username:'demo',password:'',error:'账号或密码不正确，请重试。',busy:false};
  const busyPrompt={host:'protected.example.com',username:'demo',password:'',error:'',busy:true};
  $('#auth-error-example').append(authForm(errorPrompt,{submit:()=>feedback('认证错误样例。使用交互原型可切换认证结果'),cancel:()=>feedback('已点击取消认证样例')},'error-example'));
  $('#auth-busy-example').append(authForm(busyPrompt,{cancel:()=>feedback('已点击取消验证样例')},'busy-example'));
  $('#menu-example').querySelectorAll('button').forEach(node=>node.addEventListener('click',()=>feedback(`托盘菜单样例：${node.textContent.trim()}`)));
  showColors(); document.addEventListener('themechange',showColors);
})();
