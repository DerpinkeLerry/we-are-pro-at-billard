let modulePromise;
export function getThree(){
  if(!modulePromise){ modulePromise=import(window.__POOL_BOOTSTRAP__.threeUrl); }
  return modulePromise;
}
