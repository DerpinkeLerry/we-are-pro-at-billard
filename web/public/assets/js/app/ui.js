export const escapeHtml = s => String(s ?? '').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
export function toast(message,type='info',ttl=3200){
  const root=document.querySelector('#toast-root'); if(!root)return;
  const el=document.createElement('div'); el.className=`toast ${type}`; el.textContent=message; root.append(el);
  setTimeout(()=>el.remove(),ttl);
}
export function formatDuration(ms){ if(!ms)return '—'; const s=Math.round(ms/1000); return `${Math.floor(s/60)}:${String(s%60).padStart(2,'0')}`; }
export function qs(sel,root=document){return root.querySelector(sel)}
export function qsa(sel,root=document){return [...root.querySelectorAll(sel)]}
export function modal(html){ const root=qs('#modal-root'); root.innerHTML=`<div class="modal-backdrop"><section class="modal">${html}</section></div>`; const close=()=>root.innerHTML=''; root.querySelector('.modal-backdrop').addEventListener('click',e=>{if(e.target.classList.contains('modal-backdrop'))close()}); return close; }
