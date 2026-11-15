// Server order is counterclockwise. Local rotation must preserve neighbors.
export function seats(players,you){
 const self=players.findIndex(p=>p.id===you),n=players.length;
 if(self<0)return [];
 const paired=n%2===0,sideCount=Math.floor((n-1)/2),rows=sideCount+(paired?1:0);
 return Array.from({length:n},(_,offset)=>{
  const opposite=paired&&offset===n/2;
  const side=offset===0?'self':opposite?'opposite':offset<=sideCount?'right':'left';
  const row=offset===0?rows+1:opposite?1:side==='right'?rows-offset+1:offset-sideCount;
  const angle=Math.PI/2-offset*2*Math.PI/n;
  const ring=n===6?[[50,75],[83,75],[83,25],[50,25],[17,25],[17,75]]:null;
  return {player:players[(self+offset)%n],offset,side,row,column:side==='self'||opposite?'1 / -1':side==='right'?'2':'1',x:ring?ring[offset][0]:50+Math.cos(angle)*33,y:offset===0?75:opposite?25:ring?ring[offset][1]:50+Math.sin(angle)*31};
 });
}

// Sort a copy: unknown own stones remain unknown; the server hand is untouched.
export function sortedHand(hand){return [...hand].sort((a,b)=>a-b)}
