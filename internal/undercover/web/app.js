'use strict';
const $=id=>document.getElementById(id);
let room=null,busy=false,loading=false,revision=0;
const message=text=>{ $('message').textContent=text;$('message').hidden=!text; };
const hideWord=()=>{$('secret').hidden=true;$('word').textContent='';$('covered').hidden=false;$('reveal').setAttribute('aria-pressed','false');};
async function api(path,data){
 const response=await fetch('/undercover/api/'+path,data===undefined?{cache:'no-store'}:{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});
 const result=await response.json();if(!response.ok)throw Error(result.error||'请求失败');return result;
}
function code(){return location.hash.slice(1).toUpperCase();}
function render(next){
 if(!room||room.round!==next.round||room.dealt!==next.dealt||room.code!==next.code)hideWord();
 const configChanged=!room||room.code!==next.code||room.dealt!==next.dealt||room.host!==next.host;
 room=next;$('lobby').hidden=true;$('room').hidden=false;
 $('room-code').textContent=room.code;$('round').textContent=room.dealt?'第 '+room.round+' 局 · 已发词':'等待玩家';
 $('player-count').textContent=room.players.length;
 $('players').replaceChildren(...room.players.map(p=>{
  const li=document.createElement('li'),name=document.createElement('span'),badge=document.createElement('span');
  name.className='name';name.textContent=p.name;badge.className='badge';badge.textContent=[p.you?'你':'',p.host?'房主':''].filter(Boolean).join(' · ');li.append(name,badge);return li;
 }));
 $('deal').hidden=!room.host||room.dealt;$('waiting').hidden=room.host||room.dealt;$('card').hidden=!room.dealt;$('reset').hidden=!room.host||!room.dealt;
 if(configChanged){$('room-spies').value=room.spies;$('room-blanks').value=room.blanks;}
 civilianCount();
}
function civilianCount(){if(room)$('civilian-count').textContent='平民 '+Math.max(0,room.players.length-Number($('room-spies').value)-Number($('room-blanks').value))+' 人';}
$('room-spies').oninput=civilianCount;$('room-blanks').oninput=civilianCount;
async function action(task){
 if(busy)return;busy=true;revision++;message('');
 document.querySelectorAll('button').forEach(b=>b.disabled=true);
 try{await task();}catch(e){message(e.message);}finally{busy=false;document.querySelectorAll('button').forEach(b=>b.disabled=false);}
}
function enter(next){location.hash=next.code;render(next);}
$('create').onsubmit=e=>{e.preventDefault();action(async()=>enter(await api('create',{name:$('host-name').value})));};
$('join').onsubmit=e=>{e.preventDefault();action(async()=>enter(await api('join',{name:$('guest-name').value,code:$('code').value})));};
$('deal').onsubmit=e=>{e.preventDefault();action(async()=>render(await api('deal',{code:room.code,round:room.round,spies:Number($('room-spies').value),blanks:Number($('room-blanks').value)})));};
$('reveal').onclick=()=>{
 if(!$('secret').hidden){hideWord();return;}
 return action(async()=>{
 const activeRoom=room.code,round=room.round,result=await api('word',{code:activeRoom,round});
 if(!room||room.code!==activeRoom||room.round!==round||!room.dealt)return;
 $('word').textContent=result.blank?'白板':result.word;$('covered').hidden=true;$('secret').hidden=false;$('reveal').setAttribute('aria-pressed','true');
});
};
let pendingConfirmation=null;
function askConfirmation(title,task){
 hideWord();pendingConfirmation=task;$('confirm-title').textContent=title;$('confirm-dialog').showModal();
}
$('confirm-dialog').onclose=()=>{pendingConfirmation=null;};
$('confirm-cancel').onclick=()=>{$('confirm-dialog').close();pendingConfirmation=null;};
$('confirm-accept').onclick=()=>{
 const task=pendingConfirmation;pendingConfirmation=null;$('confirm-dialog').close();
 if(task)return action(task);
};
$('reset').onclick=()=>{
 const request={code:room.code,round:room.round};
 askConfirmation('结束本局，开启下一局？',async()=>render(await api('reset',request)));
};
$('leave').onclick=()=>action(async()=>{await api('leave',{code:room.code});room=null;hideWord();location.hash='';$('room').hidden=true;$('lobby').hidden=false;});
$('share').onclick=()=>action(async()=>{
 const link=location.origin+'/undercover/#'+room.code;
 try{await navigator.clipboard.writeText(link);$('share').textContent='已复制';setTimeout(()=>$('share').textContent='复制邀请',2000);}catch{message(link);}
});
async function sync(){
 if(busy||loading||document.hidden)return;
 const current=code();if(!/^[A-F0-9]{6}$/.test(current))return;
 loading=true;const started=revision;
 try{const next=await api('room?code='+current);if(code()===current&&!busy&&revision===started){render(next);message('');}}
 catch(e){if(!room){$('code').value=current;message(e.message);}else{hideWord();message(e.message);}}
 finally{loading=false;}
}
window.addEventListener('hashchange',()=>{if(room&&room.code!==code()){room=null;hideWord();$('room').hidden=true;$('lobby').hidden=false;}sync();});
document.addEventListener('visibilitychange',()=>{hideWord();if(!document.hidden)sync();});
window.addEventListener('pagehide',hideWord);
api('info').then(info=>$('bank-count').textContent=info.pairs.toLocaleString('zh-CN')+' 组词对').catch(()=>{});
$('code').value=code();sync();setInterval(sync,2500);
