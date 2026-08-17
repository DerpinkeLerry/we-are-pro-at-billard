export class AudioManager{
  constructor(settings){this.settings=settings;this.ctx=null;this.started=false}
  update(settings){this.settings=settings}
  ensure(){ if(!this.ctx)this.ctx=new (window.AudioContext||window.webkitAudioContext)(); if(this.ctx.state==='suspended')this.ctx.resume(); this.started=true }
  tone(freq,duration=.08,volume=.15,type='sine',delay=0){ if(!this.settings.sound)return; this.ensure(); const t=this.ctx.currentTime+delay, o=this.ctx.createOscillator(), g=this.ctx.createGain(),mix=this.settings.masterVolume*this.settings.sfxVolume; o.type=type;o.frequency.setValueAtTime(freq,t);g.gain.setValueAtTime(Math.max(.0001,volume*mix),t);g.gain.exponentialRampToValueAtTime(.0001,t+duration);o.connect(g).connect(this.ctx.destination);o.start(t);o.stop(t+duration+.02) }
  event(e){ const impact=Math.max(0,Math.min(1,e.intensity||0)),v=Math.min(.3,.045+impact*.16),jitter=(Math.random()-.5)*12; if(e.type==='ball')this.tone(300+impact*115+jitter,.035,v,'triangle'); else if(e.type==='cushion'||e.type==='jaw')this.tone(125+impact*70+jitter*.35,.05,v*.8,'square'); else if(e.type==='pocket')this.tone(82+impact*35,.14,.15+impact*.05,'sine'); }
  ui(kind){ const m={cue:[105,.045,.18],queue:[510,.09,.12],turn:[720,.08,.11],foul:[180,.25,.15],win:[620,.16,.13],loss:[140,.22,.13],start:[420,.10,.11]}; const x=m[kind];if(!x)return;this.tone(x[0],x[1],x[2],kind==='foul'?'sawtooth':kind==='cue'?'triangle':'sine');if(kind==='win')this.tone(820,.2,.13,'sine',.14) }
}
