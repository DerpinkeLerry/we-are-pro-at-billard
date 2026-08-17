const STATES=['table','falling','pocketed','off_table'];

export function decodePhysicsSnapshot(buffer){
  const v=new DataView(buffer);let o=0;
  if(v.byteLength<26)return null;
  const magic=String.fromCharCode(v.getUint8(o++),v.getUint8(o++),v.getUint8(o++),v.getUint8(o++));if(magic!=='PLS1')return null;
  const seq=Number(v.getBigUint64(o,true));o+=8;const serverTime=Number(v.getBigInt64(o,true));o+=8;const simTime=v.getFloat32(o,true);o+=4;
  const idLen=v.getUint8(o++);if(o+idLen+1>v.byteLength)return null;const matchId=new TextDecoder().decode(new Uint8Array(buffer,o,idLen));o+=idLen;const count=v.getUint8(o++),balls=[];
  for(let i=0;i<count;i++){
    if(o+35>v.byteLength)return null;const id=v.getUint8(o++),state=STATES[v.getUint8(o++)]||'table',pocketId=v.getInt8(o++);const f=[];for(let n=0;n<8;n++){f.push(v.getFloat32(o,true));o+=4}
    balls.push({id,state,pocketId,x:f[0],y:f[1],z:f[2],vx:f[3],vy:f[4],wx:f[5],wy:f[6],wz:f[7]});
  }
  return {type:'PHYSICS_SNAPSHOT',seq,serverTime,data:{matchId,simTime,balls}};
}

export class GameClient extends EventTarget{
  constructor({code,getTicket}){this.code=code;this.getTicket=getTicket;this.ws=null;this.closed=false;this.reconnectAttempt=0;this.ping=0;this.clockOffset=0;this.lastSeq=0;this.timer=null;this.connecting=null;this.snapshotRate=0;this.snapshotCount=0;this.snapshotTick=performance.now();this.lastPhysicsSimTime=0}
  async connect(){if(this.connecting)return this.connecting;this.connecting=this._connect().finally(()=>this.connecting=null);return this.connecting}
  async _connect(){
    this.lastSeq=0;const {token,wsUrl}=await this.getTicket();return new Promise((resolve,reject)=>{
      const ws=new WebSocket(wsUrl);ws.binaryType='arraybuffer';this.ws=ws;let authed=false;
      const fail=setTimeout(()=>{if(!authed){try{ws.close()}catch{}reject(new Error('auth_timeout'))}},7000);
      ws.onopen=()=>ws.send(JSON.stringify({type:'AUTH',token}));
      ws.onmessage=e=>{
        let m;if(e.data instanceof ArrayBuffer)m=decodePhysicsSnapshot(e.data);else{try{m=JSON.parse(e.data)}catch{return}}if(!m)return;
        if(m.seq&&m.seq<=this.lastSeq)return;if(m.seq)this.lastSeq=m.seq;if(m.serverTime)this.clockOffset=m.serverTime-Date.now();
        if(m.type==='AUTH_OK'){authed=true;clearTimeout(fail);this.reconnectAttempt=0;this.startPing();resolve(m.data)}
        if(m.type==='AUTH_FAILED'){clearTimeout(fail);reject(new Error(m.data?.reason||'auth_failed'))}
        if(m.type==='PONG'&&m.data?.clientTime)this.ping=Math.max(0,Date.now()-m.data.clientTime);
        if(m.type==='PHYSICS_SNAPSHOT'){this.lastPhysicsSimTime=m.data?.simTime||0;this.snapshotCount++;const now=performance.now();if(now-this.snapshotTick>=1000){this.snapshotRate=Math.round(this.snapshotCount*1000/(now-this.snapshotTick));this.snapshotCount=0;this.snapshotTick=now}}
        this.dispatchEvent(new CustomEvent('message',{detail:m}));this.dispatchEvent(new CustomEvent(m.type,{detail:m.data}));
      };
      ws.onerror=()=>{};ws.onclose=()=>{clearTimeout(fail);this.stopPing();this.dispatchEvent(new Event('disconnected'));if(!this.closed)this.scheduleReconnect()};
    })
  }
  scheduleReconnect(){const delay=Math.min(4000,350*Math.pow(1.7,this.reconnectAttempt++));setTimeout(async()=>{if(this.closed)return;try{await this.connect();this.dispatchEvent(new Event('reconnected'))}catch{this.scheduleReconnect()}},delay)}
  send(type,data={}){if(this.ws?.readyState!==WebSocket.OPEN)return false;this.ws.send(JSON.stringify({type,...data}));return true}
  startPing(){this.stopPing();const ping=()=>this.send('CLIENT_PING',{clientTime:Date.now()});ping();this.timer=setInterval(ping,10000)}stopPing(){clearInterval(this.timer);this.timer=null}
  serverNow(){return Date.now()+this.clockOffset}
  close(){this.closed=true;this.stopPing();this.ws?.close()}
}
