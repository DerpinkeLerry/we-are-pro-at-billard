export const GRAPHICS = {
  'very-high': {name:'Very High',dpr:2,shadows:true,segments:32,antialias:true,fps:60,roughness:.32,envLike:true},
  normal: {name:'Normal',dpr:1.5,shadows:true,segments:20,antialias:true,fps:60,roughness:.5,envLike:false},
  'very-low': {name:'Very Low',dpr:1,shadows:false,segments:10,antialias:false,fps:30,roughness:.8,envLike:false},
};
export function normalizedPreset(v){ return GRAPHICS[v] ? v : 'normal'; }
