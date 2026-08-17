const defaults = {cueSkin:'classic-maple', graphics:'normal', sound:true, masterVolume:.65, sfxVolume:.8, aimGuide:true, aimSensitivity:1, mutedPlayers:[], devDebug:false};
let value = {...defaults};
try { value = {...defaults, ...JSON.parse(localStorage.getItem('pool-settings') || '{}')}; } catch {}
export const settings = {
  get(){ return {...value, mutedPlayers:[...(value.mutedPlayers||[])]}; },
  set(patch){ value={...value,...patch}; localStorage.setItem('pool-settings',JSON.stringify(value)); window.dispatchEvent(new CustomEvent('pool-settings',{detail:value})); return value; },
  toggleMute(id){ const s=new Set(value.mutedPlayers||[]); s.has(id)?s.delete(id):s.add(id); return this.set({mutedPlayers:[...s]}); },
};
