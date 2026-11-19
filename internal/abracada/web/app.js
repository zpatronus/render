import {seats,sortedHand} from './layout.mjs';
const $=id=>document.getElementById(id),names=['远古巨龙','黑暗幽灵','甜蜜的梦','猫头鹰','闪电风暴','冰晶术','火球术','治疗药水'],arts=['dragon','ghost','dream','owl','storm','ice','fire','potion'],effects=['其他所有人失去骰子点数生命','其他人失去1生命，自己恢复1','恢复骰子点数生命','拿取一个秘法石板','左右邻各失去1生命','左邻失去1生命','右邻失去1生命','恢复1生命'];let room,stream,busy=false,last=0,gameID='',pending,snapshot='',owlKey='',knownTablets=[],knownTabletKey='',tabletRequest=null;
const el=(tag,text,cls)=>{const e=document.createElement(tag);if(text!==undefined)e.textContent=text;if(cls)e.className=cls;return e},img=a=>{const e=el('img');e.src='/abracada/art/'+a+'.svg';e.alt='';return e},code=()=>location.hash.slice(1).toUpperCase();
function message(s=''){$('message').textContent=s;$('message').hidden=!s}
async function api(action,data){const r=await fetch('/abracada/api/'+action,data?{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)}:{cache:'no-store'});const v=await r.json();if(!r.ok)throw Error(v.error);return v}
function connect(){
 stream?.close();if(!room||document.hidden)return;
 const activeCode=room.code,source=new EventSource('/abracada/api/events?code='+activeCode);stream=source;
 const current=()=>source===stream&&room?.code===activeCode;
 source.onmessage=e=>{if(!current())return;if(!busy)render(JSON.parse(e.data));$('connection').textContent='● 实时连接'};
 source.addEventListener('reaction',event=>{if(current())showReaction(JSON.parse(event.data))});
 source.onerror=()=>{if(current())$('connection').textContent='正在重新连接…'};
 source.addEventListener('expired',()=>{if(!current())return;source.close();room=null;snapshot='';$('entry').hidden=false;$('room').hidden=true;message('房间已过期或已关闭')});
}
async function act(action,extra={}){if(busy||presentationActive)return;busy=true;const request={code:room.code,game:room.game?.id,step:room.game?.step,...extra};try{if(action==='cast')await focusSeatForCast();const result=await api(action,request);if(action!=='leave')await render(result);message()}catch(e){message(e.message)}finally{busy=false;if(action!=='leave')connect()}}
function confirm(title,fn,label='确认施法',effect=''){$('confirm-body').textContent=effect;$('confirm-title').textContent=title;$('confirm-accept').textContent=label;pending=fn;$('confirm-dialog').showModal()}
$('confirm-accept').onclick=()=>{const f=pending;$('confirm-dialog').close();f?.()};for(const id of ['confirm-close','confirm-cancel'])$(id).onclick=()=>$('confirm-dialog').close();for(const kind of ['rules','tablets']){$(kind+'-close').onclick=()=>$(kind+'-dialog').close()}$('rules-open').onclick=()=>$('rules-dialog').showModal();
try{$('name').value=localStorage.getItem('abr-name')||'';}catch{}
$('entry-form').onsubmit=async e=>{e.preventDefault();try{const nickname=$('name').value.trim();const next=await api(e.submitter?.dataset.entry||'create',{name:nickname,code:$('code').value.trim()});try{localStorage.setItem('abr-name',nickname);}catch{}location.hash=next.code;render(next);connect()}catch(e){message(e.message)}};
let presentationChain=Promise.resolve(),presentationActive=false,presentationGame='',presentationThrough=0,presentationEpoch=0,scheduledSnapshot='',resolvedRollView=null,rollWaiter=null,pendingRollPresentation='';
function render(next){
 const encoded=JSON.stringify(next);if(encoded===scheduledSnapshot)return presentationChain;
 const game=next.game;
 if(rollWaiter&&game?.id===presentationGame&&game.phase!=='rolling'){
  const rolled=game.events.find(e=>e.id===rollWaiter.eventID&&e.roll>0);
  if(rolled){resolvedRollView=next;rollWaiter.resolve(next);rollWaiter=null;}
 }
 if(!game||presentationGame!==game.id||!room?.game){
  presentationEpoch++;presentationGame=game?.id||'';presentationThrough=Math.max(0,...(game?.events||[]).map(e=>e.id));presentationActive=false;scheduledSnapshot=encoded;$('cast-stage').hidden=true;presentationChain=Promise.resolve();renderNow(next);return presentationChain;
 }
 const casts=game.events.filter(e=>e.kind==='cast'&&e.id>presentationThrough);
 const passes=game.events.filter(e=>e.kind==='pass'&&e.id>presentationThrough);
 presentationThrough=Math.max(presentationThrough,...game.events.map(e=>e.id));scheduledSnapshot=encoded;
 const epoch=presentationEpoch;
 presentationChain=presentationChain.then(async()=>{
  if(epoch!==presentationEpoch)return;
  presentationActive=true;
  try{
   for(const event of casts){
    showCastStage(event,game);
    await new Promise(resolve=>setTimeout(resolve,1000));
    if(epoch!==presentationEpoch)return;
    $('cast-stage').dataset.result=event.success?'success':'backfire';
    $('cast-stage-result').textContent=event.success?'施法成功':'反噬';
    $('cast-stage').setAttribute('aria-label',event.success?'施法成功':'施法反噬');
    if(next.game.phase==='rolling'&&(event.roll||0)===0){
     if(next.game.turn===room.you){
      const value=await drawRoll(event,epoch);
      if(epoch!==presentationEpoch)return;
      const resolved=await api('roll',{code:next.code,game:next.game.id,step:next.game.step,roll:value});
      resolvedRollView=resolved;
     }else{
      resolvedRollView=await waitForRoll(event,epoch);
     }
     if(epoch!==presentationEpoch)return;
     next=resolvedRollView||next;resolvedRollView=null;
    }
    Object.assign(event,next.game?.events?.find(e=>e.id===event.id)||{});
    if(event.roll){
     $('cast-stage-result').textContent=(event.success?'施法成功':'反噬')+'（投到 '+event.roll+' 点）';
    }
    await new Promise(resolve=>setTimeout(resolve,event.roll?1400:750));
    if(!event.success)await new Promise(resolve=>setTimeout(resolve,1000));
    if(epoch!==presentationEpoch)return;
   }
   if(!casts.length&&passes.length)await new Promise(resolve=>setTimeout(resolve,350));
   $('cast-stage').hidden=true;
   const backfire=casts.find(e=>!e.success);
   if(backfire&&room?.game?.id===next.game.id){
    const impact=structuredClone(next),before=room.game;
    impact.game.turn=before.turn;impact.game.phase='playing';impact.game.deck=before.deck;impact.game.canEnd=false;
    for(const player of impact.game.players){const previous=before.players.find(p=>p.id===player.id);if(previous)player.hand=[...previous.hand]}
    renderNow(impact);
    await new Promise(resolve=>setTimeout(resolve,450));if(epoch!==presentationEpoch)return;
   }
   renderNow(next);
   if(casts.length){if(['round-end','finished'].includes(game.phase)){$('round-result').hidden=true;await new Promise(resolve=>setTimeout(resolve,1320));if(epoch===presentationEpoch)$('round-result').hidden=false}}
  }finally{if(epoch===presentationEpoch){presentationActive=false;$('cast-stage').hidden=true}}
 }).catch(error=>{if(epoch===presentationEpoch){presentationActive=false;$('cast-stage').hidden=true;message(error.message)}});
 return presentationChain;
}
function showCastStage(event,game){
 const stage=$('cast-stage');stage.dataset.result='pending';
 $('cast-stage-name').textContent=game.players.find(p=>p.id===event.actor)?.name||'';
 $('cast-stage-spell').textContent=names[event.spell-1];
 $('cast-stage-number').textContent=game.ids[event.spell-1];
 $('cast-stage-number').className='spell-number spell-'+event.spell;
 $('cast-stage-art').replaceChildren(img(arts[event.spell-1]));
 $('cast-draw').hidden=true;$('cast-stage-result').textContent='?';stage.setAttribute('aria-label','正在施放'+names[event.spell-1]+'，等待结果');stage.dataset.player=event.actor;stage.hidden=false;positionCastCallout();
}
function drawRoll(event,epoch){
 const wrap=$('cast-draw'),canvas=$('cast-draw-pad'),ctx=canvas.getContext('2d'),button=$('cast-draw-button');
 wrap.hidden=false;ctx.clearRect(0,0,canvas.width,canvas.height);ctx.lineWidth=8;ctx.lineCap='round';ctx.lineJoin='round';ctx.strokeStyle='#e7d59e';
 const points=[];let drawing=false;
 const finish=()=>{
  if(epoch!==presentationEpoch)return;
  canvas.onpointerdown=canvas.onpointermove=canvas.onpointerup=canvas.onpointercancel=null;button.onclick=null;wrap.hidden=true;
  let seed=Date.now()>>>0;for(const p of points)seed=Math.imul(seed^p.x,16777619)>>>0,seed=Math.imul(seed^p.y,16777619)>>>0,seed=Math.imul(seed^p.t,16777619)>>>0;
  return resolve(1+seed%3);
 };
 let resolve;const result=new Promise(r=>resolve=r);
 canvas.onpointerdown=e=>{e.preventDefault();drawing=true;canvas.setPointerCapture(e.pointerId);const r=canvas.getBoundingClientRect(),p={x:Math.round((e.clientX-r.left)*canvas.width/r.width),y:Math.round((e.clientY-r.top)*canvas.height/r.height),t:Math.round(performance.now())};points.push(p);ctx.beginPath();ctx.moveTo(p.x,p.y)};
 canvas.onpointermove=e=>{if(!drawing)return;const r=canvas.getBoundingClientRect(),p={x:Math.round((e.clientX-r.left)*canvas.width/r.width),y:Math.round((e.clientY-r.top)*canvas.height/r.height),t:Math.round(performance.now())};points.push(p);ctx.lineTo(p.x,p.y);ctx.stroke()};
 canvas.onpointerup=e=>{if(drawing){drawing=false;finish()}};canvas.onpointercancel=()=>{if(drawing){drawing=false;finish()}};
  button.onclick=()=>{if(points.length===0)points.push({x:0,y:0,t:Date.now()>>>0});finish()};
 positionCastCallout();return result;
}
function waitForRoll(event,epoch){
 const found=room?.game?.events.find(e=>e.id===event.id&&e.roll>0);if(found)return Promise.resolve(room);
 return new Promise(resolve=>{rollWaiter={eventID:event.id,resolve};const check=()=>{if(epoch!==presentationEpoch){rollWaiter=null;resolve(null);return}setTimeout(check,250)};check()});
}
function renderNow(next){const callout=$('cast-stage');if(callout?.hidden&&callout.parentElement!==document.body)document.body.append(callout);const encoded=JSON.stringify(next);if(encoded===snapshot)return;if(room?.code!==next.code||room?.game?.id!==next.game?.id||room?.game?.step!==next.game?.step){$('confirm-dialog').close();pending=null;}if(room?.code!==next.code||room?.game?.id!==next.game?.id||room?.game?.round!==next.game?.round){$('tablets-dialog').close();$('owl-dialog').close();}snapshot=encoded;room=next;$('entry').hidden=true;$('room').hidden=false;$('room-code').textContent='魔法桌 '+room.code;$('lobby').hidden=!!room.game;$('game').hidden=!room.game;$('phase-title').textContent=room.game?'出包魔法师':'等待魔法师';if(!room.game){$('lobby-count').textContent=(room.members.length+room.bots)+' / 6';const lobbyPeople=room.members.concat(Array.from({length:room.bots},(_,i)=>({id:'preview-'+i,name:['星尘学徒','苔藓术士','月光使者','迷雾巫师','琥珀法师'][i],bot:true})));$('lobby-seats').replaceChildren(...lobbyPeople.map((p,i)=>{const c=el('article',undefined,'lobby-seat');c.append(el('span',String(i+1).padStart(2,'0'),'lobby-index'),el('h3',p.name),el('span',p.bot?'机器人':p.id===room.host?'房主':'','lobby-role'));return c}));$('bots').replaceChildren(...Array.from({length:6},(_,count)=>{const button=el('button',String(count));button.type='button';button.disabled=room.you!==room.host||count>6-room.members.length;button.setAttribute('aria-pressed',String(count===room.bots));button.onclick=()=>act('bots',{count});return button}));$('start').hidden=room.you!==room.host;$('mode-note').textContent=room.members.length+room.bots===6?'六人扩展 · 43 个魔法石 · 5 个秘法石板':'标准玩法 · 36 个魔法石';return}
const g=room.game,own=g.players.find(p=>p.id===room.you);const tabletKey=currentTabletKey();if(knownTabletKey!==tabletKey){knownTabletKey=tabletKey;knownTablets=[];tabletRequest=null;}if(own.tablets>knownTablets.length)loadKnownTablets().catch(()=>{});const by=id=>g.players.find(p=>p.id===id)?.name||'';if(gameID!==g.id){gameID=g.id;last=Math.max(0,...g.events.map(e=>e.id))}const ended=['round-end','finished'].includes(g.phase);$('round-label').textContent='第 '+g.round+' 轮';$('mode-label').textContent=g.players.length===6?'六人扩展':'标准玩法';$('turn-label').textContent=ended?'本轮结束':by(g.turn)+(g.turn===room.you?' · 轮到你':'');$('neighbors').textContent='你的左邻：'+by(own.left)+' · 右邻／下家：'+by(own.right);
$('score-strip').replaceChildren(...g.players.map(p=>{const score=el('div',undefined,'score-entry'+(p.id===room.you?' is-you':''));score.append(el('span',p.name),el('strong',p.score),el('span','/ 8','score-goal'));return score}));
renderSeats(g,by);if(!$('emoji-menu').hidden)positionEmojiMenu();
 $('action-panel').hidden=ended||g.turn!==room.you||g.phase==='rolling';$('action-title').textContent=g.turn===room.you?'选择你的法术':'等待 '+by(g.turn);$('action-help').textContent=g.canEnd?'可以继续施法，编号不能降低，也可以收手补牌。':'每回合至少尝试一次施法。';$('end-turn').disabled=g.turn!==room.you||!g.canEnd||g.phase!=='playing';$('spell-buttons').hidden=g.phase==='owl';$('spell-buttons').replaceChildren(...names.map((n,i)=>{const b=el('button',undefined,'spell-'+(i+1));b.append(el('b',g.ids[i],'spell-number'),img(arts[i]),el('strong',n));b.title=effects[i];b.disabled=g.turn!==room.you||i+1<g.minimum||g.phase!=='playing';b.onclick=()=>confirm('施放 '+g.ids[i]+' 号'+n+'？',()=>act('cast',{spell:i+1}),'确认施法',effects[i]+(i===0?'；失败时自己失去骰子点数生命':'；失败时自己失去 1 点生命'));return b}));$('secret-choice').hidden=g.phase!=='owl'||g.turn!==room.you;$('secret-slots').replaceChildren(...g.secrets.map((available,i)=>{const b=el('button','石板 '+(i+1));b.disabled=!available||g.turn!==room.you;b.onclick=()=>openOwl();return b}));
 $('deck-count').textContent='抽牌堆 '+g.deck;renderCountBoard(g);if(g.phase==='owl'&&g.turn===room.you&&owlKey!==g.id+':'+g.step){owlKey=g.id+':'+g.step;queueMicrotask(openOwl);}
$('secret-count').textContent='中央剩余 '+g.secrets.filter(Boolean).length+' 个 · 你持有 '+own.tablets+' 个';$('round-result').hidden=!ended;$('result-title').textContent=g.phase==='finished'?'全场赢家：'+g.winners.map(by).join('、'):'本轮结束';$('round-result').querySelector('.eyebrow').textContent=g.phase==='finished'?'游戏结束':'本轮结算';$('result-reason').textContent=g.phase==='finished'?'最后一轮：'+g.reason:g.reason;const resultKey=g.id+':'+g.round+':'+g.phase;if(ended&&$('round-result').dataset.resultKey!==resultKey){$('round-result').dataset.resultKey=resultKey;$('round-result').classList.remove('collapsed');$('result-toggle').textContent='收起结算';$('result-toggle').setAttribute('aria-expanded','true');}$('scoreboard').replaceChildren(...g.players.map(p=>el('div',p.name+' · 本轮 +'+p.gain+' · 累计 '+p.score+' 分')));$('next-round').textContent=g.phase==='finished'?'重新开局':'下一轮';$('next-round').hidden=room.you!==room.host;$('next-wait').textContent=room.you===room.host?'':'等待房主继续';$('history').replaceChildren(...g.events.slice().reverse().map(e=>el('li','第 '+e.round+' 轮 · '+e.text)));
for(const e of g.events){if(e.id<=last)continue;last=e.id;if(e.kind==='cast')animateCast(e)}}
$('start').onclick=()=>act('start');$('end-turn').onclick=()=>confirm('结束回合并补牌？',()=>act('end'),'结束回合');$('next-round').onclick=()=>act(room.game.phase==='finished'?'reset':'next');$('invite').onclick=async()=>{try{await navigator.clipboard.writeText(location.href);message('邀请链接已复制')}catch{message(location.href)}};$('leave').onclick=()=>confirm('离开魔法桌？游戏中保留席位。',async()=>{stream?.close();if(!room.game||room.game.phase==='finished')await act('leave');room=null;location.hash='';$('entry').hidden=false;$('room').hidden=true});$('tablets-open').onclick=async()=>{try{const v=await loadKnownTablets();$('my-tablets').replaceChildren(...v.tablets.map(k=>{const c=el('div',undefined,'spell-'+k);c.append(el('b',room.game.ids[k-1],'spell-number'),img(arts[k-1]),el('p',names[k-1]));return c}));$('tablets-dialog').showModal()}catch(e){message(e.message)}};
async function resume(){if(!/^[A-F0-9]{6}$/.test(code()))return;try{const next=await api('room?code='+code());await render(next);connect();if(next.game?.phase==='rolling')resumePendingRoll(next)}catch(e){message(e.message)}}window.addEventListener('hashchange',resume);document.addEventListener('visibilitychange',()=>{if(document.hidden){stream?.close();$('tablets-dialog').close();$('owl-dialog').close()}else resume()});window.addEventListener('pagehide',()=>stream?.close());window.addEventListener('pageshow',resume);$('code').value=code();resume();

async function resumePendingRoll(state){
 const event=state.game.events.find(e=>e.kind==='cast'&&e.actor===state.game.turn&&!e.roll);if(!event)return;
 const key=state.game.id+':'+event.id;if(pendingRollPresentation===key)return;pendingRollPresentation=key;
 const epoch=presentationEpoch;presentationActive=true;showCastStage(event,state.game);
 try{
  await new Promise(resolve=>setTimeout(resolve,1000));if(epoch!==presentationEpoch)return;
  $('cast-stage').dataset.result=event.success?'success':'backfire';$('cast-stage-result').textContent=event.success?'施法成功':'反噬';
  let final;
  if(state.you===event.actor){
   const value=await drawRoll(event,epoch);if(epoch!==presentationEpoch)return;
   final=await api('roll',{code:state.code,game:state.game.id,step:state.game.step,roll:value});
  }else final=await waitForRoll(event,epoch);
  if(!final||epoch!==presentationEpoch)return;
  Object.assign(event,final.game.events.find(e=>e.id===event.id)||{});
  $('cast-stage-result').textContent=(event.success?'施法成功':'反噬')+'（投到 '+event.roll+' 点）';
  await new Promise(resolve=>setTimeout(resolve,1400));if(epoch!==presentationEpoch)return;
  $('cast-stage').hidden=true;renderNow(final);
 }catch(error){if(epoch===presentationEpoch)message(error.message)}
 finally{pendingRollPresentation='';if(epoch===presentationEpoch){presentationActive=false;$('cast-stage').hidden=true}}
}

function animateCast(e){
 if(matchMedia('(prefers-reduced-motion: reduce)').matches)return;
 const source=document.querySelector('[data-player="'+e.actor+'"]');if(!source)return;
 const a=source.getBoundingClientRect();
 if(!e.success){shakeHit(source,'#f68060');return}
 const hits=new Map(),damage='#f68060',heal='#8fc58b',owl='#e7d59e';
 if(e.spell===2){for(const id of e.targets||[])hits.set(id,damage);hits.set(e.actor,heal)}
 else if(e.spell===3||e.spell===8)hits.set(e.actor,heal);
 else if(e.spell===4)hits.set(e.actor,owl);
 else for(const id of e.targets||[])hits.set(id,damage);
 for(const [id,color] of hits){const target=document.querySelector('[data-player="'+id+'"]');if(!target)continue;const b=target.getBoundingClientRect();const effect=img(arts[e.spell-1]);Object.assign(effect.style,{position:'fixed',width:'72px',height:'74px',left:(a.x+a.width/2-36)+'px',top:(a.y+a.height/2-37)+'px',filter:'drop-shadow(0 0 18px '+color+')'});$('effects').append(effect);const motion=effect.animate([{opacity:0,transform:'scale(.3) rotate(-25deg)'},{opacity:1,offset:.2,transform:'scale(1.2)'},{opacity:0,transform:'translate('+(b.x+b.width/2-a.x-a.width/2)+'px,'+(b.y+b.height/2-a.y-a.height/2)+'px) scale(.7) rotate(20deg)'}],{duration:900,easing:'ease-in-out'});motion.onfinish=()=>{effect.remove();const recipient=document.querySelector('[data-player="'+id+'"]');if(color===heal)healHit(recipient);else if(color===owl)flashHit(recipient,color);else shakeHit(recipient,color)}}
}

function openOwl(){
 const g=room?.game;if(!g||g.phase!=='owl'||g.turn!==room.you)return;
 $('owl-result').hidden=true;$('owl-done').hidden=true;$('owl-picks').hidden=false;
 $('owl-picks').replaceChildren(...g.secrets.map((available,i)=>{const b=el('button');b.append(img('back'));b.setAttribute('aria-label','选择秘法石板 '+(i+1));b.disabled=!available;b.onclick=()=>chooseTablet(i);return b}));
 if(!$('owl-dialog').open)$('owl-dialog').showModal();
}
async function chooseTablet(slot){
 if(busy)return;const id=room.game.id,round=room.game.round;
 await act('take',{slot});
 if(room?.game?.id!==id||room.game.round!==round||room.game.phase==='owl')return;
 $('owl-picks').hidden=true;
 try{const result=await loadKnownTablets();if(result.game!==id||result.round!==round)return;const k=result.tablets.at(-1);if(!k)return;const face=img(arts[k-1]),text=el('div',undefined,'spell-'+k);text.append(el('b',room.game.ids[k-1],'spell-number'),el('h3',names[k-1]));$('owl-result').replaceChildren(face,text);$('owl-result').hidden=false;$('owl-done').hidden=false;if(!matchMedia('(prefers-reduced-motion: reduce)').matches)face.animate([{opacity:0,transform:'rotateY(85deg) scale(.7)'},{opacity:1,transform:'rotateY(0) scale(1)'}],{duration:650,easing:'ease-out'});}catch(e){message(e.message);$('owl-dialog').close()}
}
$('owl-close').onclick=()=>$('owl-dialog').close();$('owl-done').onclick=()=>$('owl-dialog').close();

// Finish framing the table before requesting the cast, so its animation starts in view.
async function focusSeatForCast(){
 if(matchMedia('(min-width: 1100px)').matches)return;
 const button=$('end-turn');if(!button)return;
 const viewport=window.visualViewport;
 const bottom=(viewport?.height||innerHeight)+(viewport?.offsetTop||0);
 const start=window.scrollY;
 const target=Math.max(0,Math.min(document.documentElement.scrollHeight-innerHeight,start+button.getBoundingClientRect().bottom-bottom+24));
 if(Math.abs(target-start)<2)return;
 if(matchMedia('(prefers-reduced-motion: reduce)').matches){window.scrollTo({top:target,behavior:'instant'});return;}
 await new Promise(resolve=>{
  let began,frame,done=false;
  const finish=()=>{if(done)return;done=true;cancelAnimationFrame(frame);window.removeEventListener('touchstart',finish);window.removeEventListener('wheel',finish);resolve()};
  window.addEventListener('touchstart',finish,{passive:true,once:true});window.addEventListener('wheel',finish,{passive:true,once:true});
  const tick=time=>{if(done)return;began??=time;const t=Math.min(1,(time-began)/230),ease=1-Math.pow(1-t,3);window.scrollTo({top:start+(target-start)*ease,behavior:'instant'});if(t<1)frame=requestAnimationFrame(tick);else finish()};
  frame=requestAnimationFrame(tick);
 });
}

function renderCountBoard(g){const visible=Array(8).fill(0),privateCounts=Array(8).fill(0);if(knownTabletKey===currentTabletKey())for(const k of knownTablets)privateCounts[k-1]++;for(const player of g.players){if(player.id===room.you)continue;for(const kind of player.hand)if(kind)visible[kind-1]++}$('count-board').replaceChildren(...names.map((n,i)=>{const row=el('div',undefined,'count-row spell-'+(i+1));const label=el('div',undefined,'count-label');label.append(el('b',g.ids[i],'spell-number'),img(arts[i]),el('span',n),el('strong',(g.used[i]+g.removed[i]+visible[i]+privateCounts[i])+' / '+g.supply[i]));const slots=el('div',undefined,'count-slots');for(let j=0;j<g.supply[i];j++){const used=j<g.used[i]+g.removed[i],seen=!used&&j<g.used[i]+g.removed[i]+visible[i],mine=!used&&!seen&&j<g.used[i]+g.removed[i]+visible[i]+privateCounts[i];const slot=el('span',g.ids[i],'count-slot '+(used?'used':seen?'visible':mine?'known-tablet':'unknown'));slot.title=used?'已公开':seen?'对手手中可见':mine?'我的秘法石板':'未知';slot.setAttribute('aria-label',g.ids[i]+'号：'+slot.title);slots.append(slot)}row.append(label,slots);row.title='已公开 '+(g.used[i]+g.removed[i])+' · 可见 '+visible[i]+' · 我的石板 '+privateCounts[i]+' · 未知 '+(g.supply[i]-g.used[i]-g.removed[i]-visible[i]-privateCounts[i]);return row}));}

function currentTabletKey(){return room?.game?room.code+':'+room.game.id+':'+room.game.round+':'+room.you:'';}
function loadKnownTablets(){
 const key=currentTabletKey();if(!key)return Promise.reject(Error('游戏尚未开始'));
 if(tabletRequest?.key===key)return tabletRequest.promise;
 const code=room.code;
 const promise=api('tablets?code='+code).then(result=>{
  if(currentTabletKey()!==key||result.game!==room.game.id||result.round!==room.game.round)throw Error('本轮已结束，请重新查看');
  knownTabletKey=key;knownTablets=[...result.tablets];renderCountBoard(room.game);return result;
 }).finally(()=>{if(tabletRequest?.key===key)tabletRequest=null;});
 tabletRequest={key,promise};return promise;
}

function flashHit(target,color='#f68060'){
 if(!target)return;
 const flash=el('div',undefined,'damage-flash');target.append(flash);
 flash.style.setProperty('--hit-flash',color);const glow=flash.animate([{opacity:0},{opacity:1,offset:.2},{opacity:.65,offset:.5},{opacity:0}],{duration:400,easing:'ease-out'});glow.onfinish=()=>flash.remove();
}
function shakeHit(target,color='#f68060'){
 if(!target)return;flashHit(target,color);
 if(matchMedia('(prefers-reduced-motion: reduce)').matches)return;
 target.animate([{translate:'0px 0px'},{translate:'-4px 0px',offset:.25},{translate:'3px 0px',offset:.55},{translate:'-1px 0px',offset:.8},{translate:'0px 0px'}],{duration:400,easing:'ease-in-out'});
}
function healHit(target){if(!target)return;flashHit(target,'#8fc58b');const heart=target.querySelector('.heart-icon');if(heart&&!matchMedia('(prefers-reduced-motion: reduce)').matches)heart.animate([{transform:'scale(1)'},{transform:'scale(1.28)',offset:.45},{transform:'scale(1)'}],{duration:560,easing:'ease-out'})}

let emojiCatalog=null,emojiLoading=null,recentEmoji=[];
try{const saved=JSON.parse(localStorage.getItem('abr-emoji-recent')||'[]');if(Array.isArray(saved))recentEmoji=[...new Set(saved.filter(x=>typeof x==='string'))].slice(0,10)}catch{}
async function toggleEmojiMenu(){
 if(!$('emoji-menu').hidden){closeEmojiMenu();return}
 $('emoji-menu').hidden=false;document.querySelector('.emoji-toggle')?.setAttribute('aria-expanded','true');positionEmojiMenu();
 try{
  if(!emojiCatalog){emojiLoading??=api('emojis');emojiCatalog=await emojiLoading;const allowed=new Set(emojiCatalog.flatMap(g=>g.emojis));recentEmoji=recentEmoji.filter(e=>allowed.has(e));}
  if($('emoji-menu').hidden)return;drawEmojiMenu();positionEmojiMenu();
 }catch(error){emojiLoading=null;message(error.message);closeEmojiMenu()}
}
function closeEmojiMenu(){$('emoji-menu').hidden=true;document.querySelector('.emoji-toggle')?.setAttribute('aria-expanded','false')}
function emojiButton(emoji,quick=false){const b=el('button',emoji,'emoji-choice');b.type='button';b.setAttribute('aria-label','发送 '+emoji);b.onclick=()=>sendEmoji(emoji,quick);return b}
function drawQuickEmoji(){
 const row=document.querySelector('.emoji-quick');if(!row)return;
 const options=[...new Set([...recentEmoji,'😂','🤣','😭','🤡','👍'])].slice(0,5);
 row.replaceChildren(...options.map(emoji=>emojiButton(emoji,true)));
}
function drawEmojiMenu(){
 const recent=$('emoji-recent');recent.replaceChildren();
 if(recentEmoji.length){recent.append(el('h3','最近'));const grid=el('div',undefined,'emoji-grid');grid.append(...recentEmoji.map(emoji=>emojiButton(emoji)));recent.append(grid)}
 $('emoji-groups').replaceChildren(...(emojiCatalog||[]).map(group=>{const section=el('section');section.append(el('h3',group.name));const grid=el('div',undefined,'emoji-grid');grid.append(...group.emojis.map(emoji=>emojiButton(emoji)));section.append(grid);return section}));
}
async function sendEmoji(emoji,quick=false){
 if(!room?.game)return;
 const activeCode=room.code;
 try{await api('react',{code:activeCode,emoji});if(room?.code!==activeCode)return;recentEmoji=[emoji,...recentEmoji.filter(e=>e!==emoji)].slice(0,10);try{localStorage.setItem('abr-emoji-recent',JSON.stringify(recentEmoji))}catch{}if(!quick)drawQuickEmoji();}catch(error){message(error.message)}
}
function positionEmojiMenu(){
 if($('emoji-menu').hidden)return;
 const button=document.querySelector('.emoji-toggle');if(!button){closeEmojiMenu();return}
 const menu=$('emoji-menu'),r=button.getBoundingClientRect(),height=Math.min(430,innerHeight-32),width=Math.min(330,innerWidth-24);
 menu.style.width=width+'px';menu.style.maxHeight=height+'px';
 menu.style.left=Math.max(12,Math.min(innerWidth-width-12,r.right-width))+'px';
 menu.style.top=Math.max(16,Math.min(innerHeight-height-16,r.top-height-10))+'px';
}
function showReaction(value){
 if(!room?.game||!room.game.players.some(p=>p.id===value.player))return;
 const seat=document.querySelector('[data-player="'+value.player+'"]');if(!seat)return;
 const r=seat.getBoundingClientRect(),emoji=el('span',value.emoji,'floating-emoji');
 emoji.style.left=(r.left+r.width/2+(Math.random()-.5)*24)+'px';emoji.style.top=(r.top+Math.min(44,r.height/2))+'px';
 $('reactions').append(emoji);
 const reduced=matchMedia('(prefers-reduced-motion: reduce)').matches;
 const animation=emoji.animate(reduced?[{opacity:1},{opacity:1,offset:.7},{opacity:0}]:[{opacity:0,transform:'translate(-50%,0) scale(.7)'},{opacity:1,transform:'translate(-50%,-12px) scale(1)',offset:.08},{opacity:1,transform:'translate(-50%,-95px)',offset:.7},{opacity:0,transform:'translate(-50%,-160px)'}],{duration:5000,easing:'linear'});
 animation.onfinish=()=>emoji.remove();
}
$('emoji-close').onclick=closeEmojiMenu;
document.addEventListener('pointerdown',event=>{if(!$('emoji-menu').hidden&&!$('emoji-menu').contains(event.target)&&!event.target.closest('.emoji-controls'))closeEmojiMenu()});
document.addEventListener('keydown',event=>{if(event.key==='Escape')closeEmojiMenu()});
window.addEventListener('resize',positionEmojiMenu);window.addEventListener('scroll',positionEmojiMenu,{passive:true});

function positionCastCallout(){
 const stage=$('cast-stage');if(stage.hidden)return;
 const seat=document.querySelector('[data-player="'+stage.dataset.player+'"]');if(!seat)return;
 const area=seat.querySelector('.seat-content')||seat;
 if(stage.parentElement!==area)area.append(stage);
 stage.removeAttribute('style');stage.dataset.side='overlay';
 stage.style.setProperty('--callout-art-height',Math.max(34,Math.min(86,area.getBoundingClientRect().height-91))+'px');
}
window.addEventListener('resize',positionCastCallout);window.addEventListener('scroll',positionCastCallout,{passive:true});

// Keep unchanged seats and stones mounted: state updates must not replay dealing.
function renderSeats(g,by){
 const parent=$('seats');
 if(parent.dataset.game!==g.id){parent.replaceChildren();parent.dataset.game=g.id;}
 const existing=new Map([...parent.children].map(node=>[node.dataset.player,node]));
 const ordered=seats(g.players,room.you);
 for(const [index,s] of ordered.entries()){
  const p=s.player;let card=existing.get(p.id);
  if(!card){
   card=el('article');card.dataset.player=p.id;
   const content=el('div',undefined,'seat-content');content.append(el('h3'),el('div',undefined,'life'),el('div',undefined,'hand'));card.append(content);
   if(p.id===room.you){const controls=el('div',undefined,'emoji-controls'),quick=el('div',undefined,'emoji-quick');quick.setAttribute('aria-label','快捷表情');const button=el('button','☺︎','emoji-toggle');button.type='button';button.setAttribute('aria-label','发送表情');button.onclick=toggleEmojiMenu;controls.append(quick,button);card.append(controls);quick.append(...[...new Set([...recentEmoji,'😂','🤣','😭','🤡','👍'])].slice(0,5).map(emoji=>emojiButton(emoji,true)))}
  }
  existing.delete(p.id);
  card.className='seat '+(s.offset===0?'self ':s.side==='opposite'?'opposite ':'')+(p.id===g.turn&&!['round-end','finished'].includes(g.phase)?'active':'');
  card.style.gridColumn=s.column;card.style.gridRow=s.row;card.style.setProperty('--x',s.x+'%');card.style.setProperty('--y',s.y+'%');
  const heading=card.querySelector('h3'),name=p.name+(p.id===room.you?' · 你':'');const headingKey=name+':'+p.tablets;if(heading.dataset.key!==headingKey){heading.replaceChildren(document.createTextNode(name));if(p.tablets>=1){const count=el('span','秘法石板 '+p.tablets,'player-tablets');count.setAttribute('aria-label',p.tablets+' 个秘法石板');heading.append(count)}heading.dataset.key=headingKey;}
  const health=card.querySelector('.life');
  if(health.dataset.value!==String(p.life)){
   clearTimeout(health.updateTimer);
   const updateHealth=()=>{health.dataset.value=String(p.life);health.className='life '+(p.life>=5?'healthy':p.life>=3?'wounded':p.life>0?'critical':'depleted');health.setAttribute('aria-label','生命 '+p.life+' / 6');health.replaceChildren(el('span','♥︎','heart-icon'),el('strong',p.life),el('span','/ 6','life-max'));};
   const incoming=g.events.filter(e=>e.kind==='cast'&&e.id>last);
   const affected=incoming.filter(e=>e.success?((e.targets||[]).includes(p.id)||(e.actor===p.id&&[2,3,8].includes(e.spell))):e.actor===p.id);
   const delay=health.dataset.value!==undefined&&!matchMedia('(prefers-reduced-motion: reduce)').matches?Math.max(0,...affected.map(e=>e.success?1320:420)):0;
   if(delay)health.updateTimer=setTimeout(updateHealth,delay);else updateHealth();
  }
  const hand=card.querySelector('.hand'),stones=sortedHand(p.hand),signature=g.round+':'+g.supply.join(',')+':'+stones.join(',');
  if(hand.dataset.signature!==signature){
   const pool=new Map();for(const node of hand.children){const k=Number(node.dataset.kind);if(!pool.has(k))pool.set(k,[]);pool.get(k).push(node)}
   const keep=[];
   for(const k of stones){let stone=pool.get(k)?.shift();if(stone){stone.style.animation='none'}else{stone=el('div',undefined,'stone spell-'+k);stone.dataset.kind=String(k);stone.append(img(k?arts[k-1]:'back'),el('b',k?g.supply[k-1]:'?'));}
    const number=stone.querySelector('b'),label=String(k?g.ids[k-1]:'?');if(number.textContent!==label)number.textContent=label;keep.push(stone);
   }
   const retained=new Set(keep);for(const node of [...hand.children])if(!retained.has(node))node.remove();
   keep.forEach((node,i)=>{if(hand.children[i]!==node)hand.insertBefore(node,hand.children[i]||null)});
   hand.dataset.signature=signature;hand.dataset.count=String(stones.length);
  }
  const toggle=card.querySelector('.emoji-toggle');if(toggle)toggle.setAttribute('aria-expanded',String(!$('emoji-menu').hidden));
  if(parent.children[index]!==card)parent.insertBefore(card,parent.children[index]||null);
 }
 for(const node of existing.values())node.remove();
}

$('result-toggle').onclick=()=>{const collapsed=$('round-result').classList.toggle('collapsed');$('result-toggle').textContent=collapsed?'查看结算':'收起结算';$('result-toggle').setAttribute('aria-expanded',String(!collapsed));requestAnimationFrame(clampResultWindow)};

function clampResultWindow(){
 const panel=$('round-result');if(panel.hidden||!matchMedia('(min-width:1100px)').matches)return;
 const r=panel.getBoundingClientRect(),gap=12;
 const x=Math.max(gap,Math.min(r.left,innerWidth-r.width-gap)),y=Math.max(gap,Math.min(r.top,innerHeight-r.height-gap));
 panel.style.left=x+'px';panel.style.top=y+'px';panel.style.right='auto';panel.style.bottom='auto';
}
$('result-title').onpointerdown=event=>{
 if(!matchMedia('(min-width:1100px)').matches||event.button!==0)return;
 const panel=$('round-result'),handle=event.currentTarget,r=panel.getBoundingClientRect(),dx=event.clientX-r.left,dy=event.clientY-r.top;
 handle.setPointerCapture(event.pointerId);event.preventDefault();
 handle.onpointermove=e=>{panel.style.left=(e.clientX-dx)+'px';panel.style.top=(e.clientY-dy)+'px';panel.style.right='auto';panel.style.bottom='auto';clampResultWindow()};
 handle.onpointerup=handle.onpointercancel=()=>{handle.onpointermove=null;handle.onpointerup=null;handle.onpointercancel=null};
};
window.addEventListener('resize',()=>{if(!matchMedia('(min-width:1100px)').matches)$('round-result').removeAttribute('style');else clampResultWindow()});
new MutationObserver(()=>requestAnimationFrame(clampResultWindow)).observe($('round-result'),{attributes:true,attributeFilter:['hidden','class']});
