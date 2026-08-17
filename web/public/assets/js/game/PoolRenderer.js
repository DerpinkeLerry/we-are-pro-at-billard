import {getThree} from './three-loader.js';
import {GRAPHICS,normalizedPreset} from './graphics.js';

const BALL_COLORS=['#f7f5ea','#d92f3d','#d92f3d','#d92f3d','#d92f3d','#d92f3d','#d92f3d','#d92f3d','#101010','#f1c928','#f1c928','#f1c928','#f1c928','#f1c928','#f1c928','#f1c928'];
const CUE_THEMES={
  'classic-maple':{shaft:0xd7b882,butt:0x332013},'dark-walnut':{shaft:0xb99262,butt:0x3a1e10},
  'carbon-black':{shaft:0x303737,butt:0x0b0d0d},'tournament-blue':{shaft:0xd9bd82,butt:0x185f8c},
  crimson:{shaft:0xd8b47d,butt:0x8e2430},neon:{shaft:0xb9f587,butt:0x39ff93},
  'minimal-white':{shaft:0xf7f7f1,butt:0xcfcfcf},
};

export class PoolRenderer{
  constructor(canvas,tableConfig,preset='normal'){
    this.canvas=canvas;this.table=tableConfig;this.preset=normalizedPreset(preset);this.ballMeshes=new Map();this.target=new Map();this.zoom=1;this.aimAngle=0;this.pointerWorld={x:0,y:0};this.stats={fps:0,frameMs:0};this.lastFrame=performance.now();this.lastPaint=0;this.frames=0;this.fpsTick=this.lastFrame;this.debug=false;this.ready=this.init();
  }
  async init(){
    const THREE=await getThree();this.THREE=THREE;const g=GRAPHICS[this.preset];this.frameInterval=1000/g.fps;
    this.renderer=new THREE.WebGLRenderer({canvas:this.canvas,antialias:g.antialias,alpha:false,powerPreference:this.preset==='very-low'?'low-power':'high-performance'});this.renderer.setPixelRatio(Math.min(window.devicePixelRatio||1,g.dpr));this.renderer.shadowMap.enabled=g.shadows;this.renderer.shadowMap.type=THREE.PCFSoftShadowMap;this.renderer.outputColorSpace=THREE.SRGBColorSpace;const gl=this.renderer.getContext(),dbg=gl.getExtension('WEBGL_debug_renderer_info');this.rendererName=dbg?gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL):gl.getParameter(gl.RENDERER);
    this.scene=new THREE.Scene();this.scene.background=new THREE.Color(0x030706);this.angularAxis=new THREE.Vector3();this.angularStep=new THREE.Quaternion();
    const L=this.table.playingSurface.length,W=this.table.playingSurface.width;
    this.camera=new THREE.OrthographicCamera(-L*.65,L*.65,W*.72,-W*.72,.01,20);this.camera.position.set(0,0,5);this.camera.lookAt(0,0,0);this.scene.add(this.camera);
    this.raycaster=new THREE.Raycaster();this.pointer=new THREE.Vector2();this.rayPlane=new THREE.Plane(new THREE.Vector3(0,0,1),0);this.rayHit=new THREE.Vector3();
    const amb=new THREE.HemisphereLight(0xdfffee,0x102017,this.preset==='very-low'?1.2:1.5);this.scene.add(amb);
    if(g.shadows){const key=new THREE.DirectionalLight(0xffffff,2.1);key.position.set(-1.5,-1,4);key.castShadow=true;key.shadow.mapSize.set(this.preset==='very-high'?2048:1024,this.preset==='very-high'?2048:1024);key.shadow.camera.left=-2;key.shadow.camera.right=2;key.shadow.camera.top=1.5;key.shadow.camera.bottom=-1.5;this.scene.add(key)}
    this.tableGroup=new THREE.Group();this.scene.add(this.tableGroup);this.buildTable();this.buildAim();this.buildCue();this.resize();
  }
  material(color,rough=.5){const T=this.THREE;if(this.preset==='very-low')return new T.MeshBasicMaterial({color});return new T.MeshStandardMaterial({color,roughness:rough,metalness:.03})}
  physicsGeometry(){
    const c=this.table,L=c.playingSurface.length,W=c.playingSurface.width,halfL=L/2,halfW=W/2,cm=c.pockets.corner.mouth/Math.SQRT2,sm=c.pockets.side.mouth/2;
    const segments=[],pockets=[];let id=0,pid=0;const add=(a,b,kind)=>segments.push({id:id++,a,b,kind});
    add({x:-halfL+cm,y:halfW},{x:-sm,y:halfW},'cushion');add({x:sm,y:halfW},{x:halfL-cm,y:halfW},'cushion');
    add({x:-halfL+cm,y:-halfW},{x:-sm,y:-halfW},'cushion');add({x:sm,y:-halfW},{x:halfL-cm,y:-halfW},'cushion');
    add({x:-halfL,y:-halfW+cm},{x:-halfL,y:halfW-cm},'cushion');add({x:halfL,y:-halfW+cm},{x:halfL,y:halfW-cm},'cushion');
    for(const sx of [-1,1])for(const sy of [-1,1]){
      const pc=c.pockets.corner,throatWidth=pc.throatWidth,d={x:sx/Math.SQRT2,y:sy/Math.SQRT2},t={x:sy/Math.SQRT2,y:-sx/Math.SQRT2},corner={x:sx*halfL,y:sy*halfW};
      const mouthMid={x:corner.x-d.x*pc.mouth/2,y:corner.y-d.y*pc.mouth/2},throatMid={x:mouthMid.x+d.x*pc.shelf,y:mouthMid.y+d.y*pc.shelf};
      const ma={x:mouthMid.x+t.x*pc.mouth/2,y:mouthMid.y+t.y*pc.mouth/2},mb={x:mouthMid.x-t.x*pc.mouth/2,y:mouthMid.y-t.y*pc.mouth/2};
      const ta={x:throatMid.x+t.x*throatWidth/2,y:throatMid.y+t.y*throatWidth/2},tb={x:throatMid.x-t.x*throatWidth/2,y:throatMid.y-t.y*throatWidth/2};
      add(ma,ta,'jaw');add(mb,tb,'jaw');pockets.push({id:pid++,kind:'corner',mouthMid,throatMid,dir:d,tangent:t,mouthWidth:pc.mouth,throatWidth,shelf:pc.shelf,dropRadiusX:pc.dropRadiusX,dropRadiusY:pc.dropRadiusY});
    }
    for(const sy of [-1,1]){
      const pc=c.pockets.side,throatWidth=pc.throatWidth,d={x:0,y:sy},t={x:1,y:0},mouthMid={x:0,y:sy*halfW},throatMid={x:0,y:sy*(halfW+pc.shelf)};
      const ma={x:pc.mouth/2,y:mouthMid.y},mb={x:-pc.mouth/2,y:mouthMid.y},ta={x:throatWidth/2,y:throatMid.y},tb={x:-throatWidth/2,y:throatMid.y};
      add(ma,ta,'jaw');add(mb,tb,'jaw');pockets.push({id:pid++,kind:'side',mouthMid,throatMid,dir:d,tangent:t,mouthWidth:pc.mouth,throatWidth,shelf:pc.shelf,dropRadiusX:pc.dropRadiusX,dropRadiusY:pc.dropRadiusY});
    }
    return {segments,pockets,halfL,halfW};
  }
  buildTable(){
    const T=this.THREE,c=this.table,g=GRAPHICS[this.preset],L=c.playingSurface.length,W=c.playingSurface.width,rw=c.rails.visualWidth;
    const base=new T.Mesh(new T.BoxGeometry(L+rw*2+.08,W+rw*2+.08,.12),this.material(0x281b12,.72));base.position.z=-.095;base.receiveShadow=g.shadows;this.tableGroup.add(base);
    const cloth=new T.Mesh(new T.PlaneGeometry(L,W),this.material(0x086743,.72));cloth.position.z=0;cloth.receiveShadow=g.shadows;this.tableGroup.add(cloth);
    const railMat=this.material(0x4b2d19,.52),cushionMat=this.material(0x0b4934,.68);const pieces=[[L+rw*2,rw,0,W/2+rw/2],[L+rw*2,rw,0,-W/2-rw/2],[rw,W,L/2+rw/2,0],[rw,W,-L/2-rw/2,0]];
    for(const [x,y,px,py] of pieces){const m=new T.Mesh(new T.BoxGeometry(x,y,.11),railMat);m.position.set(px,py,.02);m.castShadow=g.shadows;m.receiveShadow=g.shadows;this.tableGroup.add(m)}
    this.geom=this.physicsGeometry();for(const s of this.geom.segments)this.addSegmentMesh(s.a,s.b,s.kind==='jaw'?.028:.035,.055,cushionMat);
    this.pockets=this.geom.pockets;const pocketMat=new T.MeshBasicMaterial({color:0x010302});for(const p of this.pockets){const pc=p.kind==='side'?c.pockets.side:c.pockets.corner,center={x:p.throatMid.x+p.dir.x*pc.dropRadiusX*.28,y:p.throatMid.y+p.dir.y*pc.dropRadiusX*.28};const geom=new T.CylinderGeometry(pc.dropRadiusX,pc.dropRadiusX,.045,g.segments);const m=new T.Mesh(geom,pocketMat);m.rotation.x=Math.PI/2;m.position.set(center.x,center.y,-.026);this.tableGroup.add(m)}
    this.buildDebugGeometry();
  }
  addSegmentMesh(a,b,width,height,mat){const T=this.THREE,dx=b.x-a.x,dy=b.y-a.y,len=Math.hypot(dx,dy),m=new T.Mesh(new T.BoxGeometry(len,width,height),mat);m.position.set((a.x+b.x)/2,(a.y+b.y)/2,height/2);m.rotation.z=Math.atan2(dy,dx);m.castShadow=GRAPHICS[this.preset].shadows;this.tableGroup.add(m);return m}
  buildDebugGeometry(){
    const T=this.THREE;this.debugGroup=new T.Group();this.debugGroup.visible=false;this.scene.add(this.debugGroup);this.debugBallObjects=new Map();
    const segMat=new T.LineBasicMaterial({color:0xff5f63}),throatMat=new T.LineBasicMaterial({color:0x54f2b3}),mouthMat=new T.LineBasicMaterial({color:0xffd166}),captureMat=new T.LineBasicMaterial({color:0x5aa9ff});
    this.debugBallMat=new T.LineBasicMaterial({color:0x65f5ff});this.debugSpinMat=new T.LineBasicMaterial({color:0xff65da});
    const line=(a,b,mat,z=.075)=>{const g=new T.BufferGeometry().setFromPoints([new T.Vector3(a.x,a.y,z),new T.Vector3(b.x,b.y,z)]);this.debugGroup.add(new T.Line(g,mat))};
    for(const s of this.geom.segments)line(s.a,s.b,segMat);
    for(const p of this.geom.pockets){
      const mt=p.tangent,w=p.mouthWidth/2,tw=p.throatWidth/2;line({x:p.mouthMid.x-mt.x*w,y:p.mouthMid.y-mt.y*w},{x:p.mouthMid.x+mt.x*w,y:p.mouthMid.y+mt.y*w},mouthMat,.078);line({x:p.throatMid.x-mt.x*tw,y:p.throatMid.y-mt.y*tw},{x:p.throatMid.x+mt.x*tw,y:p.throatMid.y+mt.y*tw},throatMat,.081);
      const depth=p.dropRadiusY,a={x:p.throatMid.x-mt.x*tw,y:p.throatMid.y-mt.y*tw},b={x:p.throatMid.x+mt.x*tw,y:p.throatMid.y+mt.y*tw},c={x:b.x+p.dir.x*depth,y:b.y+p.dir.y*depth},d={x:a.x+p.dir.x*depth,y:a.y+p.dir.y*depth};line(a,d,captureMat,.084);line(b,c,captureMat,.084);line(c,d,captureMat,.084);
    }
    const pts=[];for(let i=0;i<40;i++){const a=i/40*Math.PI*2;pts.push(new T.Vector3(Math.cos(a)*this.table.ball.radius,Math.sin(a)*this.table.ball.radius,0))}this.debugBallCircleGeometry=new T.BufferGeometry().setFromPoints(pts);
  }
  makeDebugLabel(id){const T=this.THREE,c=document.createElement('canvas');c.width=64;c.height=32;const x=c.getContext('2d');x.fillStyle='rgba(0,0,0,.7)';x.fillRect(0,0,64,32);x.fillStyle='#bffcff';x.font='bold 21px ui-monospace,monospace';x.textAlign='center';x.textBaseline='middle';x.fillText(String(id),32,16);const tex=new T.CanvasTexture(c),m=new T.SpriteMaterial({map:tex,depthTest:false,transparent:true}),sp=new T.Sprite(m);sp.scale.set(.055,.028,1);sp.position.set(0,.047,.02);return sp}
  ensureDebugBall(id){if(this.debugBallObjects.has(id))return;const T=this.THREE,g=new T.Group(),circle=new T.LineLoop(this.debugBallCircleGeometry,this.debugBallMat),velGeo=new T.BufferGeometry(),spinGeo=new T.BufferGeometry();velGeo.setAttribute('position',new T.BufferAttribute(new Float32Array(6),3));spinGeo.setAttribute('position',new T.BufferAttribute(new Float32Array(6),3));const velocity=new T.Line(velGeo,this.debugBallMat),spin=new T.Line(spinGeo,this.debugSpinMat),label=this.makeDebugLabel(id);g.add(circle,velocity,spin,label);this.debugGroup.add(g);this.debugBallObjects.set(id,{group:g,velocity,spin,label})}
  updateDebugBalls(){if(!this.debug)return;for(const [id,t] of this.target){this.ensureDebugBall(id);const o=this.debugBallObjects.get(id),visible=t.state!=='pocketed'&&t.state!=='off_table';o.group.visible=visible;if(!visible)continue;o.group.position.set(t.x,t.y,.086);const vp=o.velocity.geometry.attributes.position.array;vp[0]=0;vp[1]=0;vp[2]=0;vp[3]=(t.vx||0)*.12;vp[4]=(t.vy||0)*.12;vp[5]=0;o.velocity.geometry.attributes.position.needsUpdate=true;const sp=o.spin.geometry.attributes.position.array;sp[0]=0;sp[1]=0;sp[2]=.003;sp[3]=(t.wx||0)*.0025;sp[4]=(t.wy||0)*.0025;sp[5]=.003;o.spin.geometry.attributes.position.needsUpdate=true}}
  setDebug(on){this.debug=!!on;if(this.debugGroup)this.debugGroup.visible=this.debug}
  buildAim(){const T=this.THREE;this.aimLine=new T.Line(new T.BufferGeometry().setFromPoints([new T.Vector3(),new T.Vector3(1,0,0)]),new T.LineBasicMaterial({color:0xf4fff9,transparent:true,opacity:.72,depthTest:false}));this.aimLine.renderOrder=12;this.aimLine.visible=false;this.scene.add(this.aimLine);this.impactMarker=new T.Mesh(new T.RingGeometry(this.table.ball.radius*.2,this.table.ball.radius*.48,24),new T.MeshBasicMaterial({color:0xffe082,transparent:true,opacity:.95,side:T.DoubleSide,depthTest:false,depthWrite:false}));this.impactMarker.position.z=.046;this.impactMarker.renderOrder=13;this.impactMarker.visible=false;this.scene.add(this.impactMarker)}
  buildCue(){const T=this.THREE,g=GRAPHICS[this.preset];this.cueGroup=new T.Group();this.cueShaft=new T.Mesh(new T.CylinderGeometry(.008,.013,1.25,g.segments),this.material(0xd7b882,.45));this.cueShaft.position.y=-.625;this.cueShaft.castShadow=g.shadows;this.cueGroup.add(this.cueShaft);this.cueButt=new T.Mesh(new T.CylinderGeometry(.014,.019,.45,g.segments),this.material(0x332013,.4));this.cueButt.position.y=-1.45;this.cueButt.castShadow=g.shadows;this.cueGroup.add(this.cueButt);this.cueGroup.visible=false;this.scene.add(this.cueGroup)}
  setCueSkin(skin){const theme=CUE_THEMES[skin]||CUE_THEMES['classic-maple'];this.cueShaft?.material?.color?.setHex(theme.shaft);this.cueButt?.material?.color?.setHex(theme.butt)}
  makeBallTexture(id){const T=this.THREE,size=this.preset==='very-low'?64:this.preset==='normal'?96:160,c=document.createElement('canvas');c.width=size*2;c.height=size;const x=c.getContext('2d');x.fillStyle=BALL_COLORS[id]||'#f7f5ea';x.fillRect(0,0,c.width,c.height);const tex=new T.CanvasTexture(c);tex.colorSpace=T.SRGBColorSpace;tex.minFilter=T.LinearFilter;tex.magFilter=T.LinearFilter;return tex}
  ensureBalls(balls){const T=this.THREE,g=GRAPHICS[this.preset],r=this.table.ball.radius;for(const b of balls){if(this.ballMeshes.has(b.id))continue;const mat=this.preset==='very-low'?new T.MeshBasicMaterial({map:this.makeBallTexture(b.id)}):new T.MeshStandardMaterial({map:this.makeBallTexture(b.id),roughness:.22,metalness:0});const m=new T.Mesh(new T.SphereGeometry(r,g.segments,Math.max(8,Math.round(g.segments*.65))),mat);m.castShadow=g.shadows;m.receiveShadow=g.shadows;this.scene.add(m);this.ballMeshes.set(b.id,m)}}
  setBalls(balls,hard=false){this.ensureBalls(balls);for(const b of balls){this.target.set(b.id,b);const m=this.ballMeshes.get(b.id),visible=b.state!=='pocketed'&&b.state!=='off_table';if(hard)m.position.set(b.x,b.y,b.z ?? this.table.ball.radius);m.visible=visible}}
  firstAimHit(cue,angle){const dx=Math.cos(angle),dy=Math.sin(angle),diameter=this.table.ball.radius*2;let nearest=null;for(const [id,b] of this.target){if(id===0||b.state==='pocketed'||b.state==='off_table')continue;const rx=b.x-cue.x,ry=b.y-cue.y,projection=rx*dx+ry*dy;if(projection<=0)continue;const sideSquared=rx*rx+ry*ry-projection*projection;if(sideSquared>diameter*diameter)continue;const travel=projection-Math.sqrt(Math.max(0,diameter*diameter-sideSquared));if(travel<0||(nearest&&travel>=nearest.travel))continue;const ghostX=cue.x+dx*travel,ghostY=cue.y+dy*travel;nearest={travel,contactX:(ghostX+b.x)/2,contactY:(ghostY+b.y)/2}}return nearest}
  setAim(angle,guideVisible=true,power=0,cueSkin='classic-maple',cueVisible=guideVisible){this.aimAngle=angle;const cue=this.target.get(0);if(!cue){this.aimLine.visible=false;this.impactMarker.visible=false;this.cueGroup.visible=false;return}const T=this.THREE,dx=Math.cos(angle),dy=Math.sin(angle),hit=this.firstAimHit(cue,angle),halfL=this.table.playingSurface.length/2-this.table.ball.radius,halfW=this.table.playingSurface.width/2-this.table.ball.radius,tx=dx>0?(halfL-cue.x)/dx:dx<0?(-halfL-cue.x)/dx:Infinity,ty=dy>0?(halfW-cue.y)/dy:dy<0?(-halfW-cue.y)/dy:Infinity,railTravel=Math.max(0,Math.min(tx,ty)),endX=hit?hit.contactX:cue.x+dx*railTravel,endY=hit?hit.contactY:cue.y+dy*railTravel;this.aimLine.geometry.setFromPoints([new T.Vector3(cue.x+dx*this.table.ball.radius,cue.y+dy*this.table.ball.radius,.044),new T.Vector3(endX,endY,.044)]);this.aimLine.visible=guideVisible;if(this.impactMarker){this.impactMarker.visible=!!(guideVisible&&hit);if(hit)this.impactMarker.position.set(hit.contactX,hit.contactY,.046)}this.setCueSkin(cueSkin);const pull=Math.max(0,Math.min(1,power))*.22,gap=this.table.ball.radius+.008+pull;this.cueGroup.position.set(cue.x-dx*gap,cue.y-dy*gap,.055);this.cueGroup.rotation.z=angle-Math.PI/2;this.cueGroup.visible=cueVisible}
  screenToTable(clientX,clientY){const rect=this.canvas.getBoundingClientRect();this.pointer.x=((clientX-rect.left)/rect.width)*2-1;this.pointer.y=-((clientY-rect.top)/rect.height)*2+1;this.raycaster.setFromCamera(this.pointer,this.camera);this.raycaster.ray.intersectPlane(this.rayPlane,this.rayHit);return{x:this.rayHit.x,y:this.rayHit.y}}
  setZoom(delta){this.zoom=Math.max(.72,Math.min(1.8,this.zoom*delta));this.resize()}
  resize(){if(!this.renderer)return;const rect=this.canvas.getBoundingClientRect(),w=Math.max(1,rect.width),h=Math.max(1,rect.height),aspect=w/h,L=this.table.playingSurface.length,W=this.table.playingSurface.width;let viewW=L*1.18/this.zoom,viewH=viewW/aspect;if(viewH<W*1.35/this.zoom){viewH=W*1.35/this.zoom;viewW=viewH*aspect}this.camera.left=-viewW/2;this.camera.right=viewW/2;this.camera.top=viewH/2;this.camera.bottom=-viewH/2;this.camera.updateProjectionMatrix();this.renderer.setSize(w,h,false)}
  render(now=performance.now()){
    if(!this.renderer||now-this.lastPaint<this.frameInterval-.5)return;const rawFrame=this.lastPaint?now-this.lastPaint:this.frameInterval,dt=Math.min(.05,(now-this.lastFrame)/1000);this.lastFrame=now;this.lastPaint=now;this.stats.frameMs=this.stats.frameMs?this.stats.frameMs*.86+rawFrame*.14:rawFrame;
    for(const [id,t] of this.target){const m=this.ballMeshes.get(id);if(!m)continue;const z=t.z ?? this.table.ball.radius,dx=t.x-m.position.x,dy=t.y-m.position.y,dz=z-m.position.z,dist=Math.hypot(dx,dy,dz),a=dist>.11?1:1-Math.exp(-dt*22);m.position.x+=dx*a;m.position.y+=dy*a;m.position.z+=dz*a;const wx=t.wx||0,wy=t.wy||0,wz=t.wz||0,angularSpeed=Math.hypot(wx,wy,wz);if(angularSpeed>1e-7){this.angularAxis.set(wx/angularSpeed,wy/angularSpeed,wz/angularSpeed);this.angularStep.setFromAxisAngle(this.angularAxis,angularSpeed*dt);m.quaternion.premultiply(this.angularStep)}m.visible=t.state!=='pocketed'&&t.state!=='off_table'}
    this.updateDebugBalls();this.renderer.render(this.scene,this.camera);this.frames++;if(now-this.fpsTick>=500){this.stats.fps=Math.round(this.frames*1000/(now-this.fpsTick));this.frames=0;this.fpsTick=now}
  }
  rendererInfo(){const i=this.renderer?.info;return i?{calls:i.render.calls,triangles:i.render.triangles,geometries:i.memory.geometries,textures:i.memory.textures,dpr:this.renderer.getPixelRatio(),renderer:this.rendererName}:{} }
  destroy(){this.renderer?.dispose();for(const m of this.ballMeshes.values()){m.geometry?.dispose();m.material?.map?.dispose();m.material?.dispose()}this.debugGroup?.traverse?.(o=>{o.geometry?.dispose?.();o.material?.map?.dispose?.();o.material?.dispose?.()})}
}
