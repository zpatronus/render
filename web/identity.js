(()=>{
 const nativeFetch=window.fetch.bind(window),NativeEvents=window.EventSource;
 let saved={};try{saved=JSON.parse(localStorage.getItem('sidequests-room-login')||'{}')}catch{}
 const base='/'+location.pathname.split('/').filter(Boolean)[0],key=base;let pair=saved[key]||null,lastRoom=pair?.room||'';
 function randomPIN(){const n=new Uint32Array(1);do{crypto.getRandomValues(n)}while(n[0]>=4294960000);return String(n[0]%10000).padStart(4,'0')}
 const initialPIN=pair?.pin||randomPIN();
 function remember(value){pair=value;saved[key]=value;try{localStorage.setItem('sidequests-room-login',JSON.stringify(saved))}catch{}}
 function fields(){return [...document.querySelectorAll('[data-room-username]')]}
 window.fetch=async(input,options)=>{
  const url=new URL(typeof input==='string'?input:input.url,location.href);
  if(url.origin!==location.origin||!url.pathname.startsWith(base+'/api/'))return nativeFetch(input,options);
  let data;try{data=JSON.parse(options?.body||'null')}catch{}
  const entry=data&&(/\/(create|join)$/.test(url.pathname)||url.pathname.endsWith('/api/rooms'));
  let chosen=pair;
  if(entry){const name=(url.pathname.endsWith('/api/rooms')?data.player:data.name)?.trim();const field=fields().find(e=>e.value.trim()===name);chosen={name,pin:field?.closest('form')?.querySelector('.room-pin')?.value||initialPIN};}
  if(chosen){
   if(data){data.username=chosen.name;data.pin=chosen.pin;options={...options,body:JSON.stringify(data)}}else{url.searchParams.set('username',chosen.name);url.searchParams.set('pin',chosen.pin);input=url.href}
  }
  const response=await nativeFetch(input,options);
  if(entry&&response.ok){try{const result=await response.clone().json();lastRoom=result.code||result.room?.id||''}catch{}remember({...chosen,room:lastRoom})}
  return response;
 };
 window.EventSource=class extends NativeEvents{constructor(input,options){const url=new URL(input,location.href);if(url.origin===location.origin&&url.pathname.startsWith(base+'/api/')&&pair){url.searchParams.set('username',pair.name);url.searchParams.set('pin',pair.pin)}super(url.href,options)}};
 function changeCurrentPIN(){
  if(!pair||!lastRoom)return;
  let dialog=document.getElementById('room-pin-dialog');
  if(!dialog){
   dialog=document.createElement('dialog');dialog.id='room-pin-dialog';dialog.className='app-dialog';dialog.style.cssText='width:360px;max-width:calc(100vw - 32px);padding:24px;background:var(--surface,#252d23);color:var(--ink,#f1efe7);border:1px solid var(--edge,#7c8771);border-radius:3px';
   dialog.innerHTML='<form><h2 style="font-size:24px;margin:0 0 20px">修改本房间 PIN</h2><label>新的 4 位 PIN<input class="new-room-pin" inputmode="numeric" pattern="[0-9]{4}" maxlength="4" autocomplete="off" required style="display:block;width:100%;margin:8px 0 16px;padding:10px;background:transparent;color:inherit;border:1px solid #7c8771"></label><p class="pin-error" role="status" style="font-size:14px;color:#e78379"></p><div style="display:flex;justify-content:flex-end;gap:10px"><button type="button" class="pin-cancel">取消</button><button type="submit" class="primary">保存</button></div></form>';
   dialog.querySelector('.pin-cancel').onclick=()=>dialog.close();
   dialog.querySelector('form').onsubmit=async event=>{
    event.preventDefault();const newPin=dialog.querySelector('input').value,error=dialog.querySelector('.pin-error'),button=dialog.querySelector('[type=submit]');
    if(!/^\d{4}$/.test(newPin)){error.textContent='请输入 4 位数字';return}
    button.disabled=true;
    try{const response=await nativeFetch(base+'/api/pin',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({code:lastRoom,username:pair.name,pin:pair.pin,newPin})});const result=await response.json();if(!response.ok)throw Error(result.error);remember({...pair,pin:newPin});for(const field of document.querySelectorAll('.room-pin'))field.value=newPin;dialog.close()}catch(problem){error.textContent=problem.message}finally{button.disabled=false}
   };
   document.body.append(dialog);
  }
  dialog.querySelector('input').value=pair.pin;dialog.querySelector('.pin-error').textContent='';dialog.showModal();
 }

 function setup(){for(const id of ['name','host-name','guest-name','join-name']){
  const field=document.getElementById(id);if(!field)continue;field.dataset.roomUsername='true';if(pair)field.value=pair.name;
  const label=document.createElement('label');label.textContent='4 位 PIN';const pin=document.createElement('input');pin.className='room-pin';pin.type='text';pin.inputMode='numeric';pin.pattern='[0-9]{4}';pin.maxLength=4;pin.autocomplete='off';pin.value=initialPIN;
  const change=document.createElement('button');change.type='button';change.className='quiet';change.textContent='修改 PIN';change.style.cssText='margin-top:6px;min-height:28px;padding:4px 8px;font-size:12px';
  const status=document.createElement('span');status.setAttribute('role','status');status.style.cssText='display:block;font-size:12px;margin-top:4px';
  change.onclick=async()=>{const name=field.value.trim(),newPin=pin.value;if(!/^\d{4}$/.test(newPin)){status.textContent='请输入 4 位数字';return}
   const code=lastRoom||location.hash.slice(1);if(!pair||pair.name!==name||!code){remember({name,pin:newPin});status.textContent='已记住，下次加入房间时使用';return}
   try{const response=await nativeFetch(base+'/api/pin',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({code,username:name,pin:pair.pin,newPin})});const result=await response.json();if(!response.ok)throw Error(result.error);remember({name,pin:newPin,room:code});for(const input of document.querySelectorAll('.room-pin'))input.value=newPin;status.textContent='本房间 PIN 已修改'}catch(error){status.textContent=error.message}
  };
  label.append(pin,change,status);field.insertAdjacentElement('afterend',label);
 }const actions=document.getElementById('leave')?.parentElement;if(actions){const button=document.createElement('button');button.type='button';button.className='quiet';button.textContent='修改 PIN';button.onclick=changeCurrentPIN;actions.append(button)}}
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup);else setup();
})();
