import {getThree} from './three-loader.js';

const THEMES={
  'classic-maple':{shaft:0xd8b47e,butt:0x4a2b18,wrap:0xefe7d4},
  'dark-walnut':{shaft:0xb68a5a,butt:0x26150d,wrap:0x111111},
  'carbon-black':{shaft:0x28302f,butt:0x070909,wrap:0x7cf3c8},
  'tournament-blue':{shaft:0xd7b77a,butt:0x155a87,wrap:0x102f47},
  crimson:{shaft:0xd1aa74,butt:0x8a2431,wrap:0x321319},
  neon:{shaft:0xc7e99a,butt:0x2ae488,wrap:0x0b3421},
  'minimal-white':{shaft:0xf0eee8,butt:0xd8d8d4,wrap:0x171717},
};

export class CuePreview{
  constructor(canvas,cue='classic-maple'){this.canvas=canvas;this.cue=cue;this.destroyed=false;this.ready=this.init()}
  async init(){
    const T=await getThree();this.T=T;this.renderer=new T.WebGLRenderer({canvas:this.canvas,antialias:true,alpha:true,powerPreference:'low-power'});this.renderer.setPixelRatio(Math.min(devicePixelRatio||1,1.5));this.scene=new T.Scene();this.camera=new T.PerspectiveCamera(32,2.5,.01,10);this.camera.position.set(0,-2.2,1.05);this.camera.lookAt(0,0,0);this.scene.add(new T.HemisphereLight(0xffffff,0x142018,2.1));const key=new T.DirectionalLight(0xffffff,2);key.position.set(-2,-2,3);this.scene.add(key);this.group=new T.Group();this.scene.add(this.group);this.build();this.resize();this.loop();window.addEventListener('resize',this.onResize=()=>this.resize());
  }
  mat(color){return new this.T.MeshStandardMaterial({color,roughness:.36,metalness:.04})}
  build(){
    if(!this.group)return;for(const c of [...this.group.children]){c.geometry?.dispose();c.material?.dispose();this.group.remove(c)}const T=this.T,t=THEMES[this.cue]||THEMES['classic-maple'];
    const piece=(r1,r2,len,color,x)=>{const m=new T.Mesh(new T.CylinderGeometry(r1,r2,len,24),this.mat(color));m.rotation.z=Math.PI/2;m.position.x=x;this.group.add(m)};
    piece(.024,.030,.82,t.butt,-.64);piece(.021,.024,.28,t.wrap,-.09);piece(.010,.021,1.18,t.shaft,.64);piece(.011,.011,.035,0xf2efe5,1.247);piece(.011,.011,.025,0x3a77a6,1.277);this.group.rotation.x=.12;
  }
  setCue(cue){this.cue=THEMES[cue]?cue:'classic-maple';this.build()}
  resize(){if(!this.renderer)return;const r=this.canvas.getBoundingClientRect(),w=Math.max(1,r.width),h=Math.max(1,r.height);this.camera.aspect=w/h;this.camera.updateProjectionMatrix();this.renderer.setSize(w,h,false)}
  loop=()=>{if(this.destroyed)return;this.group.rotation.z+=.0035;this.renderer.render(this.scene,this.camera);this.raf=requestAnimationFrame(this.loop)}
  destroy(){this.destroyed=true;cancelAnimationFrame(this.raf);window.removeEventListener('resize',this.onResize);this.group?.traverse(o=>{o.geometry?.dispose();o.material?.dispose?.()});this.renderer?.dispose()}
}
