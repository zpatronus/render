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
 const context={document:{hidden:false,activeElement:null,getElementById:$,createElement:tag=>new Element(tag),addEventListener:(e,fn)=>events.set(e,fn)},window:{addEventListener:(e,fn)=>events.set(e,fn)},location:{hash:'#ABC123',origin:'http://example.test'},navigator:{},localStorage:{getItem:()=>null,setItem(){}},requestAnimationFrame:()=>0,setInterval:fn=>{poll=fn;},setTimeout:()=>{},fetch:async(url,options)=>{
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
 const h=await harness(),{$,requests}=h;
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
 const h=await harness(),{$,state,requests}=h;
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
 $('identity-toggle').onclick();await h.flush();assert.equal($('identity-dialog').open,true);assert.equal($('identity').querySelector('h3').textContent,'希特勒');assert.match($('identity').querySelector('img').src,/role-hitler\.png$/);
 $('identity-close').onclick();assert.equal($('identity').hidden,true);
 $('identity-toggle').onclick();await h.flush();assert.equal($('identity-dialog').open,true,'identity should reopen on the first click');
 state.game.phase='finished';state.game.step=20;state.game.actions=[];state.game.winner='fascist';state.game.reason='测试';
 const roles=['hitler','liberal','fascist','liberal','liberal'];state.game.results=state.members.map((p,i)=>({...p,role:roles[i],party:i===0||i===2?'fascist':'liberal'}));
 await h.poll();assert.equal($('identity-dialog').open,false);
 for(const card of $('result-cards').children){const index=Number(card.dataset.player.slice(1));assert.ok(card.querySelector('.result-role').className.includes('role-'+roles[index]));assert.match(card.querySelector('img').src,new RegExp('role-'+roles[index]));}
 const first=$('result-cards').children[0];state.host='p1';await h.poll();assert.equal($('result-cards').children[0],first,'host transfer must not rebuild or mismatch final roles');
});

test('unchanged polls do not replay card animations and stale confirmations are dismissed',async()=>{
 const h=await harness(),{$,state}=h;
 const first=$('liberal-track').children[0];state.game.voteCount=1;await h.poll();assert.equal($('liberal-track').children[0],first);
 $('controls').children[0].children[0].onclick();assert.equal($('confirm-dialog').open,true);
 state.game.step++;state.game.phase='nomination';state.game.actions=[];await h.poll();assert.equal($('confirm-dialog').open,false);
 state.game.liberalPolicies=1;await h.poll();assert.match($('liberal-track').children[0].className,/is-new/);
});
