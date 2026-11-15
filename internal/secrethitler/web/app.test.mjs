import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';

class Element {
 constructor(tag='div'){this.tagName=tag;this.children=[];this.dataset={};this.attributes={};this.style={setProperty(k,v){this[k]=v;}};this.hidden=false;this.open=false;this.value='';this.textContent='';this.className='';this.classList={add:(c)=>{this.className+=' '+c;}};}
 append(...children){this.children.push(...children);}
 replaceChildren(...children){this.children=[...children];}
 setAttribute(k,v){this.attributes[k]=v;}
 showModal(){this.open=true;}
 close(){const was=this.open;this.open=false;if(was)this.onclose?.();}
 querySelector(selector){const match=e=>selector.startsWith('.')?e.className.split(' ').includes(selector.slice(1)):e.tagName===selector;for(const c of this.children){if(match(c))return c;const nested=c.querySelector(selector);if(nested)return nested;}return null;}
}
async function harness(){
 const html=await readFile(new URL('./index.html',import.meta.url),'utf8');
 const source=await readFile(new URL('./app.js',import.meta.url),'utf8');
 const elements=new Map();for(const m of html.matchAll(/<(\w+)\b[^>]*\bid="([^"]+)"[^>]*>/g)){const e=new Element(m[1]);e.hidden=/\bhidden\b/.test(m[0]);elements.set(m[2],e);}
 const $=id=>{assert.ok(elements.has(id),'missing DOM id '+id);return elements.get(id);};
 const members=Array.from({length:5},(_,i)=>({id:'p'+i,name:'玩家'+i,avatar:i+1,alive:true,bot:i>0}));
 const state={code:'ABC123',host:'p0',you:'p0',bots:4,members,game:{id:'game-1',step:1,phase:'voting',election:1,president:'p1',nominee:'p0',liberalPolicies:0,fascistPolicies:0,tracker:0,deckCount:17,discardCount:0,power:'',track:['','','peek','execute','execute',''],voteCount:0,actions:['vote'],targets:[],events:[]}};
 let poll;const events=new Map(),requests=[];
 const context={queueMicrotask,document:{hidden:false,activeElement:null,getElementById:$,createElement:tag=>new Element(tag),addEventListener:(e,fn)=>events.set(e,fn)},window:{addEventListener:(e,fn)=>events.set(e,fn)},location:{hash:'#ABC123',origin:'http://example.test'},navigator:{},localStorage:{getItem:()=>null,setItem(){}},setInterval:fn=>{poll=fn;},setTimeout:()=>{},fetch:async(url,options)=>{
  const body=options?.body?JSON.parse(options.body):null;requests.push({url,body});
  let data;
  if(url.includes('/private?'))data={game:state.game.id,step:state.game.step,id:'p0',role:'hitler',party:'fascist',known:[],investigations:[],peek:[],hand:state.game.phase==='president-discard'?['liberal','fascist','fascist']:[]};
  else {if(url.endsWith('/vote')){state.game.actions=[];state.game.voteCount++;}data=structuredClone(state);}
  return {ok:true,json:async()=>data};
 }};
 vm.runInNewContext(source,context);const flush=()=>new Promise(setImmediate);await flush();
 return {$,state,requests,poll:()=>poll(),events,flush,html,source};
}

test('all avatars are unlocked and votes require explicit custom confirmation',async()=>{
 const h=await harness(),{$,requests}=h;$('identity-close').onclick();
 assert.equal($('avatars').children.length,20);
 assert.doesNotMatch(h.html,/id="ready"/);
 assert.doesNotMatch(h.source,/\b(?:confirm|alert|prompt)\s*\(/);
 const row=$('controls').children[0];assert.equal(row.children[0].attributes['aria-label'],'Ja!');assert.equal(row.children[1].attributes['aria-label'],'Nein!');
 const posted=()=>requests.filter(r=>r.url.endsWith('/vote'));
 row.children[0].onclick();assert.equal($('confirm-dialog').open,true);assert.equal($('confirm-accept').textContent,'Ja!');assert.equal(posted().length,0);
 $('confirm-cancel').onclick();assert.equal(posted().length,0);
 row.children[1].onclick();await $('confirm-accept').onclick();
 assert.equal(posted().length,1);assert.equal(posted()[0].body.yes,false);
});

test('card selection is deliberate, preserves its index, and needs a second confirmation',async()=>{
 const h=await harness(),{$,state,requests}=h;$('identity-close').onclick();
 state.game.phase='president-discard';state.game.step=2;state.game.actions=['discard'];await h.poll();
 await $('controls').children[0].onclick();
 const hand=$('confirm-body').children[0];assert.equal(hand.children.length,3);assert.equal($('confirm-accept').hidden,true);
 hand.children[1].onclick();assert.equal($('confirm-accept').hidden,false);assert.match($('confirm-title').textContent,/弃置这张法西斯政策/);
 assert.equal(requests.some(r=>r.url.endsWith('/discard')),false);
 await $('confirm-accept').onclick();
 const post=requests.find(r=>r.url.endsWith('/discard'));assert.equal(post.body.card,1);assert.equal(post.body.game,'game-1');assert.equal(post.body.step,2);
});

test('identity modal and final role artwork use the authoritative role by player ID',async()=>{
 const h=await harness(),{$,state}=h;
 assert.equal($('identity-dialog').open,true,'show a covered role popup on entry');
 assert.equal($('identity').hidden,true);assert.equal(h.requests.some(r=>r.url.includes('/private?')),false,'entry must not reveal or fetch the role');
 $('identity-reveal').onclick();await h.flush();assert.equal($('identity').querySelector('h3').textContent,'希特勒');assert.match($('identity').querySelector('img').src,/role-hitler\.png$/);
 $('identity-close').onclick();assert.equal($('identity').hidden,true);
 $('identity-toggle').onclick();assert.equal($('identity-dialog').open,true);assert.equal($('identity').hidden,true,'reopening should start covered');
 $('identity-reveal').onclick();await h.flush();
 state.game.phase='finished';state.game.step=20;state.game.actions=[];state.game.winner='fascist';state.game.reason='测试';
 const roles=['hitler','liberal','fascist','liberal','liberal'];state.game.results=state.members.map((p,i)=>({...p,role:roles[i],party:i===0||i===2?'fascist':'liberal'}));
 await h.poll();assert.equal($('identity-dialog').open,false);
 for(const card of $('result-cards').children){const index=Number(card.dataset.player.slice(1));assert.ok(card.querySelector('.result-role').className.includes('role-'+roles[index]));assert.match(card.querySelector('img').src,new RegExp('role-'+roles[index]));}
 const first=$('result-cards').children[0];state.host='p1';await h.poll();assert.equal($('result-cards').children[0],first,'host transfer must not rebuild or mismatch final roles');
});

test('unchanged polls do not replay card animations and stale confirmations are dismissed',async()=>{
 const h=await harness(),{$,state}=h;$('identity-close').onclick();
 const first=$('liberal-track').children[0];state.game.voteCount=1;await h.poll();assert.equal($('liberal-track').children[0],first);
 $('controls').children[0].children[0].onclick();assert.equal($('confirm-dialog').open,true);
 state.game.step++;state.game.phase='nomination';state.game.actions=[];await h.poll();assert.equal($('confirm-dialog').open,false);
 state.game.liberalPolicies=1;await h.poll();assert.match($('liberal-track').children[0].className,/is-new/);
});

test('important public events queue once and never reveal executed roles',async()=>{
 const h=await harness(),{$,state}=h;
 state.game.events=[{id:1,kind:'policy',policy:'fascist',election:1,text:'颁布法西斯政策'}];await h.poll();
 assert.equal($('notice-dialog').open,false,'identity popup takes priority');
 $('identity-close').onclick();await h.flush();
 assert.equal($('notice-dialog').open,true);assert.equal($('notice-title').textContent,'颁布法西斯政策');assert.match($('notice-body').querySelector('img').src,/policy-fascist/);
 state.game.events.push({id:2,kind:'execution',target:'p2',election:2,text:'玩家2被处决'});await h.poll();
 assert.equal($('notice-title').textContent,'颁布法西斯政策','new events should not replace the current notice');
 $('notice-done').onclick();await h.flush();
 assert.equal($('notice-title').textContent,'玩家2 被处决');assert.match($('notice-body').querySelector('img').src,/avatar-3/);assert.equal($('notice-body').querySelector('.result-role'),null);
 $('notice-done').onclick();await h.flush();await h.poll();assert.equal($('notice-dialog').open,false,'polling must not replay a consumed event');
});

test('both trackers reset together, remaining counts follow policies, and history keeps named ballots',async()=>{
 const h=await harness(),{$,state}=h;
 assert.equal($('remaining-liberal').textContent,'自由派 6 张');
 assert.equal($('remaining-fascist').textContent,'法西斯 11 张');
 const start=[$('election-marker').style.left,$('status-marker').style.left];
 state.game.tracker=2;state.game.liberalPolicies=2;state.game.fascistPolicies=3;
 state.game.events=[{id:1,kind:'vote',election:1,text:'政府选举未通过',president:'p1',chancellor:'p0',votes:[{id:'p0',yes:true},{id:'p1',yes:false}]}];
 await h.poll();
 assert.equal($('tracker').textContent,'流局计数 2 / 3');
 assert.notEqual($('election-marker').style.left,start[0]);assert.notEqual($('status-marker').style.left,start[1]);
 assert.equal($('remaining-liberal').textContent,'自由派 4 张');assert.equal($('remaining-fascist').textContent,'法西斯 8 张');
 const votes=$('history').children[0].querySelector('.history-votes');
 assert.equal(votes.children[0].textContent,'玩家0：Ja!');assert.equal(votes.children[1].textContent,'玩家1：Nein!');
 state.game.tracker=0;state.game.lastVotes=[{id:'p0',yes:false}];await h.poll();
 assert.equal($('election-marker').style.left,start[0]);assert.equal($('status-marker').style.left,start[1]);
 assert.equal($('history').children[0].querySelector('.history-votes').children[0].textContent,'玩家0：Ja!');
});
