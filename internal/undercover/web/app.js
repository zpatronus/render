'use strict';
const $=id=>document.getElementById(id);
let room=null,busy=false,loading=false,revision=0,rosterKey='';
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
 room=next;connectStream();$('lobby').hidden=true;$('room').hidden=false;
 $('room-code').textContent=room.code;$('round').textContent=room.dealt?'第 '+room.round+' 局 · 已发词':'等待玩家';
 $('player-count').textContent=room.players.length;
 renderVoting();
 $('deal').hidden=!room.host||room.dealt;$('waiting').hidden=room.host||room.dealt;$('card').hidden=!room.dealt;$('reset').hidden=!room.host||!room.dealt;
 if(configChanged){$('room-spies').value=room.spies;$('room-blanks').value=room.blanks;}
 civilianCount();
}

function ballotKey() {
 return JSON.stringify([room.code,room.round,room.vote?.number]);
}

function renderVoting() {
 const vote=room.vote;
 const me=room.players.find(p=>p.you);
 const canVote=Boolean(vote?.open && me && !me.role && !vote.choice);
 if(voteConfirmation && (!canVote || voteConfirmation!==ballotKey())){
  pendingConfirmation=null;
  voteConfirmation=null;
  $('confirm-dialog').close();
 }
 $('start-vote').hidden=!vote?.canStart;
 let status='';
 if(vote?.open){
  status=`投票中 · ${vote.cast} / ${vote.eligible} 人已投`;
  if(me?.role)status+=' · 你已出局';
  else if(vote.choice)status+=` · 已投给 ${vote.choice}`;
  else status+=' · 请选择一位玩家';
 }else if(vote?.result){
  const result=vote.result;
  if(result.kind==='eliminated')status=`${result.name} 出局 · ${result.role}`;
  else if(result.kind==='tie')status='最高票数并列，本次无人出局。';
  else if(result.kind==='cancelled')status='有玩家离开，本次投票已取消。';
 }
 const summary=$('vote-status');
 if(summary.textContent!==status)summary.textContent=status;
 summary.hidden=!status;

 const nextKey=JSON.stringify([room.code,room.round,vote?.number,canVote,room.players]);
 if(rosterKey===nextKey)return;
 rosterKey=nextKey;
 $('players').replaceChildren(...room.players.map(p=>{
  const li=document.createElement('li');
  const name=document.createElement('span');
  name.className='name';
  name.textContent=p.name;
  const badge=document.createElement('span');
  badge.className='badge';
  badge.textContent=[p.you?'你':'',p.host?'房主':'',p.role?`出局 · ${p.role}`:''].filter(Boolean).join(' · ');
  if(p.role)li.className='eliminated';
  li.append(name,badge);
  if(canVote && !p.you && !p.role){
   const button=document.createElement('button');
   button.type='button';
   button.className='vote-choice';
   button.textContent='投票';
   button.setAttribute('aria-label','投票给 '+p.name);
   button.onclick=()=>confirmVote(p.name);
   li.append(button);
  }
  return li;
 }));
}

function confirmVote(target) {
 const request={code:room.code,round:room.round,ballot:room.vote.number,target};
 askConfirmation(`投票给「${target}」？`,async()=>render(await api('vote',request)),{
  note:'确认后，本次投票无法更改。',
  label:'确认投票',
  ballot:ballotKey(),
 });
}

$('start-vote').onclick=()=>action(async()=>{
 if(!room?.vote?.canStart)return;
 render(await api('start-vote',{code:room.code,round:room.round,ballot:room.vote.number}));
});
function civilianCount(){if(room)$('civilian-count').textContent='普通人 '+Math.max(0,room.players.length-Number($('room-spies').value)-Number($('room-blanks').value))+' 人';}
$('room-spies').oninput=civilianCount;$('room-blanks').oninput=civilianCount;
async function action(task){
 if(busy)return;busy=true;revision++;message('');
 document.querySelectorAll('button').forEach(b=>b.disabled=true);
 try{await task();}catch(e){message(e.message);}finally{busy=false;connectStream(true);document.querySelectorAll('button').forEach(b=>b.disabled=false);}
}
try{const nickname=localStorage.getItem('undercover-name')||'';$('host-name').value=nickname;$('guest-name').value=nickname;}catch{}
function enter(next){const nickname=next.players.find(p=>p.you)?.name;if(nickname){try{localStorage.setItem('undercover-name',nickname);}catch{}$('host-name').value=nickname;$('guest-name').value=nickname;}location.hash=next.code;render(next);}
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
let pendingConfirmation=null,voteConfirmation=null;
function askConfirmation(title,task,{note='',label='确认',ballot=null}={}){
 hideWord();
 pendingConfirmation=task;
 voteConfirmation=ballot;
 $('confirm-title').textContent=title;
 $('confirm-note').textContent=note;
 $('confirm-note').hidden=!note;
 $('confirm-accept').textContent=label;
 $('confirm-dialog').showModal();
}
function clearConfirmation(){
 pendingConfirmation=null;
 voteConfirmation=null;
}
$('confirm-dialog').onclose=()=>{if(!$('confirm-dialog').open)clearConfirmation();};
$('confirm-cancel').onclick=()=>{clearConfirmation();$('confirm-dialog').close();};
$('confirm-accept').onclick=()=>{
 const task=pendingConfirmation;clearConfirmation();$('confirm-dialog').close();
 if(task)return action(task);
};
$('reset').onclick=()=>{
 const request={code:room.code,round:room.round};
 askConfirmation('结束本局，开启下一局？',async()=>render(await api('reset',request)));
};
$('leave').onclick=()=>action(async()=>{await api('leave',{code:room.code});stopStream();room=null;hideWord();location.hash='';$('room').hidden=true;$('lobby').hidden=false;});
$('share').onclick=()=>action(async()=>{
 const link=location.origin+'/undercover/#'+room.code;
 try{await navigator.clipboard.writeText(link);$('share').textContent='已复制';setTimeout(()=>$('share').textContent='复制邀请',2000);}catch{message(link);}
});

let roomStream=null,streamCode='';
function stopStream(){if(roomStream){roomStream.close();roomStream=null;}streamCode='';}
function connectStream(force=false){
 const current=code();
 if(document.hidden||!/^[A-F0-9]{6}$/.test(current)||typeof EventSource==='undefined'){stopStream();return;}
 if(!force&&roomStream&&streamCode===current)return;
 stopStream();streamCode=current;
 const source=new EventSource('/undercover/api/events?code='+current);roomStream=source;
 source.onmessage=event=>{if(roomStream!==source||code()!==current||busy)return;try{const next=JSON.parse(event.data);revision++;render(next);message('');}catch{message('实时更新失败，请重新连接');}};
 source.addEventListener('expired',()=>{if(roomStream!==source)return;stopStream();room=null;hideWord();$('room').hidden=true;$('lobby').hidden=false;$('code').value=current;message('房间已关闭或你已离开房间');});
 source.onerror=()=>{if(roomStream!==source)return;message('连接中断，正在重新连接…');if(source.readyState===EventSource.CLOSED){stopStream();sync();}};
}

async function sync(){
 if(busy||loading||document.hidden)return;
 const current=code();if(!/^[A-F0-9]{6}$/.test(current))return;
 loading=true;const started=revision;
 try{const next=await api('room?code='+current);if(code()===current&&!busy&&revision===started){render(next);message('');}}
 catch(e){if(!room){$('code').value=current;message(e.message);}else{hideWord();message(e.message);}}
 finally{loading=false;}
}
window.addEventListener('hashchange',()=>{stopStream();if(room&&room.code!==code()){room=null;hideWord();$('room').hidden=true;$('lobby').hidden=false;}sync();});
document.addEventListener('visibilitychange',()=>{hideWord();if(!document.hidden){sync();connectStream(true);}else stopStream();});
window.addEventListener('pagehide',()=>{hideWord();stopStream();});
api('info').then(info=>$('bank-count').textContent=info.pairs.toLocaleString('zh-CN')+' 组词对').catch(()=>{});
$('code').value=code();sync();

window.addEventListener('pageshow',()=>connectStream());
