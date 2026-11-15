'use strict';
const $=id=>document.getElementById(id),base='/secret-hitler/';
const roleNames={liberal:'自由派',fascist:'法西斯',hitler:'希特勒'};
const powerNames={investigate:'调查党派','special-election':'特别选举',peek:'查看政策',execute:'处决'};
const phaseNames={nomination:'提名总理',voting:'政府选举','president-discard':'总统弃牌','chancellor-enact':'总理立法','veto-decision':'否决议程',executive:'总统权力','power-result':'总统查看结果',finished:'本局结束'};
let room=null,busy=false,polling=false,revision=0,snapshot='',phaseKey='',avatar=1,pending=null;
let noticeGame='',lastEvent=0,noticeQueue=[],activeNotice=null;
const el=(tag,text,cls)=>{const e=document.createElement(tag);if(text!==undefined)e.textContent=text;if(cls)e.className=cls;return e;};
const image=(src,alt,cls)=>{const e=el('img',undefined,cls);e.src=base+'art/'+src;e.alt=alt;return e;};
const button=(text,fn,cls)=>{const b=el('button',text,cls);b.type='button';b.onclick=fn;return b;};
const name=id=>room?.members.find(p=>p.id===id)?.name||'—';
function notify(text=''){$('message').textContent=text;$('message').hidden=!text;}
function coverIdentity(){$('identity').replaceChildren();$('identity').hidden=true;$('identity-reveal').hidden=false;$('identity-cover-label').textContent='点击查看身份';}
function hideIdentity(){coverIdentity();$('identity-dialog').close();$('identity-toggle').setAttribute('aria-expanded','false');}
function openIdentity(){coverIdentity();$('identity-dialog').showModal();$('identity-toggle').setAttribute('aria-expanded','true');}
function closeConfirm(){pending=null;$('confirm-dialog').close();}
function confirmAction(title,body,task,label='确认'){
 $('confirm-accept').hidden=false;
 $('confirm-title').textContent=title;$('confirm-body').replaceChildren();if(body)$('confirm-body').append(body);
 $('confirm-accept').textContent=label;pending=task;$('confirm-dialog').showModal();
}
$('confirm-cancel').onclick=closeConfirm;$('confirm-dialog').onclose=()=>{pending=null;queueMicrotask(pumpNotices);};
$('confirm-accept').onclick=()=>{const task=pending;closeConfirm();if(task)return run(task);};
for(const kind of ['rules','credits','avatar']){$(kind+'-close').onclick=()=>$(kind+'-dialog').close();$(kind+'-dialog').onclose=()=>queueMicrotask(pumpNotices);}
$('identity-close').onclick=hideIdentity;
$('identity-dialog').onclose=()=>{if(!$('identity-dialog').open){coverIdentity();$('identity-toggle').setAttribute('aria-expanded','false');queueMicrotask(pumpNotices);}};
$('rules-open').onclick=()=>$('rules-dialog').showModal();$('credits-open').onclick=()=>$('credits-dialog').showModal();
async function api(endpoint,data){
 const response=await fetch(base+'api/'+endpoint,data===undefined?{cache:'no-store'}:{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});
 let result;try{result=await response.json();}catch{throw Error('服务器暂时不可用');}
 if(!response.ok){const error=Error(result.error||'请求失败');error.status=response.status;throw error;}return result;
}
async function run(task){if(busy)return;busy=true;revision++;notify();try{await task();}catch(error){notify(error.message);}finally{busy=false;}}
function roomCode(){return location.hash.slice(1).toUpperCase();}
function enter(next){location.hash=next.code;render(next);}
function entry(){noticeQueue=[];noticeGame='';lastEvent=0;activeNotice=null;$('notice-dialog').close();room=null;snapshot='';phaseKey='';hideIdentity();$('entry').hidden=false;$('room').hidden=true;}
function credentials(){return {code:room.code,game:room.game?.id,step:room.game?.step};}
async function act(action,extra={},captured=credentials()){render(await api(action,{...captured,...extra}));}
function chooseAvatar(n){avatar=n;$('avatar-preview').src=base+'art/avatar-'+n+'.svg';for(const b of $('avatars').children)b.setAttribute('aria-pressed',String(Number(b.dataset.avatar)===n));try{localStorage.setItem('sh-avatar',String(n));}catch{}}
for(let n=1;n<=20;n++){const b=button('',()=>{chooseAvatar(n);$('avatar-dialog').close();});b.dataset.avatar=n;b.setAttribute('aria-label','头像 '+n);b.append(image('avatar-'+n+'.svg','头像 '+n));$('avatars').append(b);}
try{const saved=Number(localStorage.getItem('sh-avatar'));if(saved>=1&&saved<=20)avatar=saved;$('name').value=localStorage.getItem('sh-name')||'';}catch{}
chooseAvatar(avatar);$('avatar-open').onclick=()=>$('avatar-dialog').showModal();
$('entry-form').onsubmit=event=>{
 event.preventDefault();const kind=event.submitter?.dataset.entry||'create';
 run(async()=>{const nickname=$('name').value.trim();if(!nickname||nickname.length>20)throw Error('请填写 1–20 个字的昵称');const code=$('code').value.trim().toUpperCase();if(kind==='join'&&!/^[A-F0-9]{6}$/.test(code))throw Error('请输入 6 位房间码');
 const next=await api(kind,{name:nickname,avatar,code});try{localStorage.setItem('sh-name',nickname);}catch{}enter(next);});
};
function render(next){
 const encoded=JSON.stringify(next);if(encoded===snapshot)return;
 const enteringGame=!!next.game&&room?.game?.id!==next.game.id;
 snapshot=encoded;room=next;
 const key=next.code+':'+(next.game?next.game.id+':'+next.game.step:'lobby');
 if(key!==phaseKey){if(enteringGame||!next.game||next.game.phase==='finished')hideIdentity();if($('confirm-dialog').open)closeConfirm();phaseKey=key;}
 $('entry').hidden=true;$('room').hidden=false;$('room-code').textContent=next.code+(next.game?' · 第 '+next.game.election+' 届':'');
 $('lobby').hidden=!!next.game;$('game').hidden=!next.game;$('results').hidden=!next.game||next.game.phase!=='finished';
 $('phase-title').textContent=next.game?phaseNames[next.game.phase]:'等待玩家';
 $('identity-toggle').hidden=!next.game||next.game.phase==='finished';
 if(!next.game){noticeQueue=[];noticeGame='';lastEvent=0;activeNotice=null;$('notice-dialog').close();renderLobby();return;}
 renderGame();
 if(enteringGame&&next.game.phase!=='finished')openIdentity();
 collectNotices(next.game);
}
function playerCard(p,lobby=false){
 const c=el('div',undefined,'player');c.dataset.player=p.id;c.append(image('avatar-'+p.avatar+'.svg',p.name));
 const label=el('span',p.name+(p.id===room.you?' · 你':''),'player-name');label.title=p.name;c.append(label);
 const labels=[];
 if(p.id===room.host)labels.push('房主');
 if(p.bot)labels.push('BOT');
 if(!lobby){
  if(!p.alive){labels.push('已处决');c.classList.add('is-dead');}
  else if(p.id===room.game.president){labels.push('总统');c.classList.add('is-president');}
  else if(p.id===room.game.nominee){labels.push('总理');c.classList.add('is-nominee');}
  if(room.game.phase==='voting'&&p.alive){const dot=el('span',undefined,'vote-dot'+(p.voted?' voted':''));dot.setAttribute('aria-label',p.voted?'已投票':'待投票');dot.title=p.voted?'已投票':'待投票';c.append(dot);}
 }
 c.append(el('div',labels.join(' · '),'player-label'));
 if(lobby&&!p.bot&&room.host===room.you&&p.id!==room.you){const kick=button('×',()=>{const request={code:room.code,target:p.id};confirmAction('移除 '+p.name+'？',null,async()=>render(await api('kick',request)));},'quiet kick');kick.setAttribute('aria-label','移除 '+p.name);c.append(kick);}
 return c;
}
function renderSeats(parent,people,lobby=false){
 const existing=new Set([...parent.children].map(e=>e.dataset.player));
 parent.replaceChildren(...people.map(p=>{const card=playerCard(p,lobby);if(existing.has(p.id))card.style.animation='none';return card;}));
}
function renderLobby(){
 $('lobby-count').textContent=(room.members.length+room.bots)+' / 10 人';
 const previews=Array.from({length:room.bots},(_,i)=>({id:'preview-'+i,name:'机器人 '+(i+1),avatar:(room.members.length+i)%20+1,bot:true,alive:true}));
 renderSeats($('lobby-players'),room.members.concat(previews),true);
 $('start').hidden=room.host!==room.you;$('start').disabled=false;
 const input=$('bot-count');input.disabled=room.host!==room.you;input.min=Math.max(0,5-room.members.length);input.max=10-room.members.length;
 if(document.activeElement!==input)input.value=room.bots;
}
$('bot-count').onchange=()=>run(()=>act('bots',{count:Number($('bot-count').value)}));
$('start').onclick=()=>run(()=>act('start'));
$('invite').onclick=()=>run(async()=>{const link=location.origin+base+'#'+room.code;try{await navigator.clipboard.writeText(link);$('invite').textContent='已复制';setTimeout(()=>$('invite').textContent='复制邀请',1800);}catch{notify(link);}});
$('leave').onclick=()=>{
 const active=room.game&&room.game.phase!=='finished',request={code:room.code};
 confirmAction(active?'离开页面？席位会保留，可用原浏览器重连。':'离开房间？',null,async()=>{if(!active)await api('leave',request);location.hash='';entry();});
};
$('rematch').onclick=()=>{const captured=credentials();confirmAction('返回房间，准备新一局？',null,()=>act('reset',{},captured));};
function renderTrack(kind,total,count){
 const parent=$(kind+'-track'),key=room.game.id;const before=parent.dataset.game===key?Number(parent.dataset.count||0):0;
 if(parent.dataset.game===key&&before===count)return;
 parent.dataset.game=key;parent.dataset.count=String(count);parent.replaceChildren();
 for(let i=1;i<=total;i++){
  const slot=el('div',undefined,'policy-slot'+(i<=count?' filled':'')+(i<=count&&i>before?' is-new':''));
  if(i<=count)slot.append(image('board-policy-'+kind+'.png',roleNames[kind]+'政策'));
  parent.append(slot);
 }

}
function roleArt(role,avatar){return role==='hitler'?'role-hitler.png':'role-'+role+'-'+(((avatar||1)-1)%(role==='liberal'?6:3)+1)+'.png';}
function renderGame(){
 const g=room.game;renderTrack('liberal',5,g.liberalPolicies);renderTrack('fascist',6,g.fascistPolicies);
 const count=room.members.length;$('fascist-board-art').src=base+'art/board-fascist-'+(count<=6?'5-6':count<=8?'7-8':'9-10')+'.png';
 $('election-marker').style.left=[35.8,44.95,54.1,63.25][g.tracker]+'%';
 $('status-marker').style.left=[21.4,40.4,59.4,78.5][g.tracker]+'%';
 $('remaining-liberal').textContent='自由派 '+(6-g.liberalPolicies)+' 张';
 $('remaining-fascist').textContent='法西斯 '+(11-g.fascistPolicies)+' 张';
 $('power-legend').replaceChildren(...g.track.map((p,i)=>el('span',(i+1)+' · '+(i===5?'获胜':powerNames[p]||'—')+(i===4?' / 否决':''))));
 $('liberal-count').textContent=g.liberalPolicies+' / 5';$('fascist-count').textContent=g.fascistPolicies+' / 6';
 $('tracker').textContent='流局计数 '+g.tracker+' / 3';$('deck-count').textContent='牌堆 '+g.deckCount;$('discard-count').textContent='弃牌 '+g.discardCount;
 renderSeats($('game-players'),room.members);
 const government=$('government');government.replaceChildren();government.append(el('div','总统：'+name(g.president)),el('div','总理：'+name(g.nominee)));
 renderControls();
 $('ballots').hidden=!g.lastVotes?.length;
 if(g.lastVotes?.length&&$('ballot-list').dataset.key!==g.lastVotePresident+g.lastVoteChancellor+JSON.stringify(g.lastVotes)){ $('ballot-list').dataset.key=g.lastVotePresident+g.lastVoteChancellor+JSON.stringify(g.lastVotes);$('ballot-title').textContent=name(g.lastVotePresident)+' ＋ '+name(g.lastVoteChancellor)+' · '+(g.lastVotePassed?'通过':'未通过');$('ballot-list').replaceChildren(...g.lastVotes.map(v=>{const b=el('span',name(v.id),'ballot '+(v.yes?'yes':'no'));b.style.setProperty('--i',String(g.lastVotes.indexOf(v)));b.append(image(v.yes?'vote-yes.png':'vote-no.png',v.yes?'Ja!':'Nein!','ballot-result-art'));return b;}));}
 $('history').replaceChildren(...g.events.map(e=>{const item=el('li','第 '+e.election+' 届 · '+e.text);if(e.votes?.length){item.append(el('div','总统：'+name(e.president)+' · 总理：'+name(e.chancellor)));const votes=el('div',undefined,'history-votes');for(const v of e.votes)votes.append(el('span',name(v.id)+'：'+(v.yes?'Ja!':'Nein!'),v.yes?'role-liberal':'role-fascist'));item.append(votes);}return item;}));
 $('identity-toggle').hidden=g.phase==='finished';
 if(g.phase==='finished')renderResults(g);
}
function renderResults(g){
 $('victory-header').src=base+'art/victory-'+g.winner+'-header.png';$('victory-footer').src=base+'art/victory-'+g.winner+'-footer.png';
 $('winner').textContent=roleNames[g.winner]+'获胜';$('winner').className='role-'+g.winner;$('win-reason').textContent=g.reason;
 // Each result is keyed by the same immutable player ID used by the private role view.
 if($('result-cards').dataset.game!==g.id){$('result-cards').dataset.game=g.id;$('result-cards').replaceChildren(...g.results.map(p=>{const c=el('article',undefined,'result-card');c.dataset.player=p.id;c.style.setProperty('--i',String(g.results.indexOf(p)));c.append(image(roleArt(p.role,p.avatar),roleNames[p.role]),el('h3',p.name+(p.id===room.you?' · 你':'')),el('div',roleNames[p.role],'result-role role-'+p.role),el('p',p.alive?'存活':'已处决','result-status'));return c;}));}
 $('rematch').hidden=room.you!==room.host;
}
function renderControls(){
 const g=room.game,controls=$('controls'),a=g.actions;const key=g.id+':'+g.step+':'+a.join(',');
 if(controls.dataset.state===key){const progress=controls.querySelector('.vote-progress');if(progress)progress.textContent=g.voteCount+' / '+room.members.filter(p=>p.alive).length+' 已提交';return;}
 controls.dataset.state=key;controls.replaceChildren();
 const titles={nominate:'选择总理候选人',vote:'投票',discard:'弃置一张政策',enact:'颁布一张政策',veto:'是否同意否决？',investigate:'选择调查对象','special-election':'指定下一位总统',execute:'选择处决对象',peek:'查看牌堆顶三张政策',acknowledge:'查看私密结果'};
 if(!a.length){const you=room.members.find(p=>p.id===room.you);$('action-title').textContent=g.phase==='finished'?'本局结束':!you?.alive?'你已被处决':g.phase==='voting'?'等待其余玩家投票':'等待 '+name(g.phase==='chancellor-enact'?g.nominee:g.president);if(g.phase==='voting')controls.append(el('p',g.voteCount+' / '+room.members.filter(p=>p.alive).length+' 已提交','vote-progress'));return;}
 $('action-title').textContent=titles[a[0]];
 if(a.includes('vote')||a.includes('veto')){const action=a.includes('vote')?'vote':'veto',capture=credentials(),row=el('div',undefined,'votes');for(const yes of [true,false]){const label=yes?'Ja!':'Nein!',art=()=>image(yes?'vote-yes.png':'vote-no.png',label);const b=button('',()=>confirmAction(action==='vote'?'确认投票':'确认否决决定',art(),()=>act(action,{yes},capture),label),yes?'ja':'nein');b.setAttribute('aria-label',label);b.append(art());row.append(b);}controls.append(row);return;}
 if(a.includes('discard')||a.includes('enact')){const action=a.includes('discard')?'discard':'enact';controls.append(button('查看政策',()=>run(()=>showHand(action)),'primary'));if(a.includes('request-veto')){const capture=credentials();controls.append(button('请求否决',()=>confirmAction('请求否决本次议程？',null,()=>act('request-veto',{},capture))));}return;}
 if(a.includes('peek')){const capture=credentials();controls.append(button('查看政策',()=>run(()=>act('peek',{},capture)),'primary'));return;}
 if(a.includes('acknowledge')){controls.append(button('查看私密结果',()=>run(showPowerResult),'primary'));return;}
 const action=a[0],capture=credentials(),targets=el('div',undefined,'target-list');
 for(const id of g.targets){const p=room.members.find(p=>p.id===id);const b=button('',()=>confirmAction(titles[action]+'：'+p.name+'？',null,()=>act(action,{target:id},capture)));b.append(image('avatar-'+p.avatar+'.svg',''),el('span',p.name));targets.append(b);}controls.append(targets);
}
async function privateData(){const code=room.code,id=room.game.id,step=room.game.step;const result=await api('private?code='+code);if(!room||room.code!==code||room.game?.id!==id||room.game?.step!==step||result.game!==id||result.step!==step)throw Error('游戏阶段已更新，请重新查看');return result;}
async function showHand(action){
 const capture=credentials(),data=await privateData(),hand=el('div',undefined,'hand '+(data.hand.length===2?'two':''));
 if(!data.hand.length)throw Error('当前没有可操作的政策');
 for(const [index,p]of data.hand.entries()){
  const b=button('',()=>{const body=image('policy-'+p+'.png',roleNames[p]+'政策');body.className='confirmation-policy';confirmAction((action==='discard'?'弃置':'颁布')+'这张'+roleNames[p]+'政策？',body,()=>act(action,{card:index},capture));});b.append(image('policy-'+p+'.png',roleNames[p]+'政策'),el('span',action==='discard'?'弃置':'颁布'));hand.append(b);
 }
 confirmAction(action==='discard'?'选择要弃置的政策':'选择要颁布的政策',hand,null,'选择政策');$('confirm-accept').hidden=true;
}
async function showPowerResult(){
 const capture=credentials(),data=await privateData(),body=el('div',undefined,'private-result');
 if(room.game.power==='investigate'){const result=data.investigations.at(-1);body.append(el('p',name(result.id)+'：'+roleNames[result.party]));const card=image('party-membership-'+result.party+'.png',roleNames[result.party]+'党派');card.className='confirmation-policy';body.append(card);}
 else{body.append(el('p','牌堆顶 →︎ 第三张'));const cards=el('div',undefined,'peek-cards');for(const p of data.peek)cards.append(image('policy-'+p+'.png',roleNames[p]+'政策'));body.append(cards);}
 confirmAction('私密结果',body,()=>act('acknowledge',{},capture),'完成');
}
$('identity-toggle').onclick=openIdentity;
$('identity-reveal').onclick=()=>{
 if(!$('identity').hidden){coverIdentity();return;}
 run(async()=>{const data=await privateData(),content=el('div',undefined,'identity-card');const documents=el('div',undefined,'identity-documents');documents.append(image(roleArt(data.role,room.members.find(p=>p.id===room.you)?.avatar),roleNames[data.role]),image('party-membership-'+data.party+'.png',roleNames[data.party]+'党派'));content.append(documents,el('h3',roleNames[data.role],'role-'+data.role));
 if(data.known.length){for(const p of data.known)content.append(el('p',name(p.id)+' · '+roleNames[p.role]));}
 else if(data.role==='hitler')content.append(el('p','本局你不知道其他法西斯是谁。'));
 if(data.investigations.length){const history=el('div',undefined,'private-history');history.append(el('h3','调查记录'));for(const r of data.investigations)history.append(el('p',name(r.id)+' · '+roleNames[r.party]));content.append(history);}
 $('identity').replaceChildren(content);$('identity').hidden=false;$('identity-reveal').hidden=true;});
};
function collectNotices(g){
 if(noticeGame!==g.id){noticeGame=g.id;lastEvent=Math.max(0,...g.events.map(e=>e.id||0));noticeQueue=[];activeNotice=null;$('notice-dialog').close();return;}
 for(const e of g.events){if(e.id>lastEvent&&['policy','execution','victory'].includes(e.kind))noticeQueue.push(e);lastEvent=Math.max(lastEvent,e.id||0);}
 pumpNotices();
}
function pumpNotices(){
 if(!room?.game||document.hidden||activeNotice||!noticeQueue.length)return;
 if(['confirm','identity','rules','credits','avatar'].some(id=>$(id+'-dialog').open))return;
 const e=noticeQueue.shift();activeNotice=e;const body=$('notice-body');body.replaceChildren();
 if(e.kind==='policy'){
  $('notice-title').textContent='颁布'+roleNames[e.policy]+'政策';body.append(image('policy-'+e.policy+'.png',roleNames[e.policy]+'政策','notice-policy'));
 }else if(e.kind==='execution'){
  const target=room.members.find(p=>p.id===e.target);$('notice-title').textContent=name(e.target)+' 被处决';
  if(target)body.append(image('avatar-'+target.avatar+'.svg',target.name,'notice-avatar'));
  body.append(el('p','该玩家已退出本局投票与任职。'));
 }else{
  $('notice-title').textContent=roleNames[e.winner]+'获胜';body.append(image('victory-'+e.winner+'-header.png',roleNames[e.winner]+'获胜','notice-victory'),el('p',e.text));
 }
 $('notice-dialog').showModal();
}
$('notice-done').onclick=()=>{$('notice-dialog').close();};
$('notice-dialog').onclose=()=>{activeNotice=null;queueMicrotask(pumpNotices);};

async function sync(){
 if(busy||polling||document.hidden)return;const code=roomCode();if(!/^[A-F0-9]{6}$/.test(code))return;
 polling=true;const started=revision;
 try{const next=await api('room?code='+code);if(roomCode()===code&&!busy&&revision===started){render(next);notify();}}
 catch(error){if(roomCode()!==code)return;if(error.status===403||error.status===404){entry();$('code').value=code;}notify(error.message);}
 finally{polling=false;}
}
window.addEventListener('hashchange',()=>{if(room?.code!==roomCode())entry();$('code').value=roomCode();sync();});
function coverSecrets(){hideIdentity();if($('confirm-dialog').open)closeConfirm();}
document.addEventListener('visibilitychange',()=>{coverSecrets();if(!document.hidden){pumpNotices();sync();}});window.addEventListener('pagehide',coverSecrets);
$('code').value=roomCode();sync();setInterval(sync,3000);
