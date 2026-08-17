export async function quickBenchmark(){
  const canvas=document.createElement('canvas'); canvas.width=640; canvas.height=360;
  const gl=canvas.getContext('webgl2',{antialias:true,alpha:false}) || canvas.getContext('webgl',{antialias:true,alpha:false});
  if(!gl) return {preset:'very-low',score:0,label:'WebGL nicht verfügbar'};
  const start=performance.now();
  for(let i=0;i<350;i++){ gl.clearColor((i%31)/31,.08,.12,1); gl.clear(gl.COLOR_BUFFER_BIT|gl.DEPTH_BUFFER_BIT); }
  gl.finish(); const ms=performance.now()-start;
  const mem=navigator.deviceMemory || 4, cores=navigator.hardwareConcurrency || 4;
  let preset='normal'; if(ms<90 && mem>=8 && cores>=6)preset='very-high'; if(ms>260 || mem<=2 || cores<=2)preset='very-low';
  const score=Math.round(100000/Math.max(ms,1));
  return {preset,score,label:`Score ${score} · ${ms.toFixed(0)} ms · ${cores} Threads${navigator.deviceMemory?` · ${mem} GB`:''}`};
}
