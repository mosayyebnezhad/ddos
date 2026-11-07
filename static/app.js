const $ = (s) => document.querySelector(s);
const fmt = n => new Intl.NumberFormat('en-US').format(n ?? 0);
function paint(s) {
  document.querySelectorAll('[data-status]').forEach(e => { e.textContent=s.status; e.dataset.state=s.availability<=5?'down':s.availability<100?'warn':'ok'; });
  document.querySelectorAll('[data-cpu]').forEach(e=>e.textContent=s.cpu+'%');
  document.querySelectorAll('[data-mem]').forEach(e=>e.textContent=s.memory_mb+' MB');
  document.querySelectorAll('[data-availability]').forEach(e=>e.textContent=s.availability+'%');
  document.querySelectorAll('[data-incoming]').forEach(e=>e.textContent=fmt(s.incoming));
  document.querySelectorAll('[data-effective]').forEach(e=>e.textContent=fmt(s.effective_incoming));
  document.querySelectorAll('[data-blocked]').forEach(e=>e.textContent=fmt(s.blocked));
  document.querySelectorAll('[data-accepted]').forEach(e=>e.textContent=fmt(s.accepted));
  document.querySelectorAll('[data-dropped]').forEach(e=>e.textContent=fmt(s.dropped));
  document.querySelectorAll('[data-total]').forEach(e=>e.textContent=fmt(s.total));
  document.querySelectorAll('[data-goroutines]').forEach(e=>e.textContent=fmt(s.goroutines));
  document.querySelectorAll('[data-attack]').forEach(e=>{e.textContent=s.attack?'ACTIVE':'STANDBY';e.dataset.state=s.attack?'down':'ok'});
  document.querySelectorAll('[data-bar="cpu"]').forEach(e=>e.style.width=s.cpu+'%');
  document.querySelectorAll('[data-bar="availability"]').forEach(e=>e.style.width=s.availability+'%');
  document.querySelectorAll('[data-bar="load"]').forEach(e=>e.style.width=Math.min(100,s.effective_incoming/s.capacity*100)+'%');
  document.querySelectorAll('[data-bar="blocked"]').forEach(e=>e.style.width=(s.attack_rps>0?Math.min(100,Math.max(0,(s.blocked/(s.total||1))*100)):0)+'%');
  const defenseState=$('[data-mitigation-state]'); if(defenseState){defenseState.textContent=s.mitigation?'فعال · RATE LIMITING ON':'غیرفعال';defenseState.dataset.state=s.mitigation?'ok':'warn';}
  const defenseButton=$('#toggle-mitigation'); if(defenseButton){defenseButton.textContent=s.mitigation?'خاموش‌کردن Rate Limiting':'فعال‌سازی Rate Limiting';defenseButton.classList.toggle('safe',!s.mitigation);}
  const defenseDescription=$('#mitigation-description'); if(defenseDescription)defenseDescription.textContent=s.mitigation?'درخواست‌های مبدأ مهاجم به ۲۰ درخواست در ثانیه محدود می‌شوند؛ ترافیک کاربران عادی حفظ می‌شود.':'دفاع خاموش است؛ تمام ترافیک شبیه‌سازی‌شده به سمت ظرفیت سرویس محاسبه می‌شود.';
  document.querySelectorAll('[data-user-message]').forEach(e=>{e.textContent=s.availability<=5?'اتصال قطع شد؛ سرویس در دسترس نیست.':s.availability<100?'اتصال ناپایدار است؛ در حال تلاش مجدد…':'در حال تماشای محتوا';});
  document.querySelectorAll('[data-user-screen]').forEach(e=>e.dataset.outage=s.availability<=5?'true':'false');
  const log=$('#event-log'); if(log && (window.lastAvailability!==s.availability || window.lastMitigation!==s.mitigation)){const row=document.createElement('div');row.className='logline';const event=s.mitigation?'RATE LIMITING '+(s.attack?'FILTERING':'ENABLED'):s.status;row.innerHTML=`<span>${new Date().toLocaleTimeString()}</span><b>${event}</b><small>availability ${s.availability}% · raw ${s.incoming} r/s · effective ${s.effective_incoming} r/s · blocked ${fmt(s.blocked)}</small>`;log.prepend(row);while(log.children.length>7)log.lastChild.remove();window.lastAvailability=s.availability;window.lastMitigation=s.mitigation;}
  const n=$('#last-update'); if(n)n.textContent=new Date().toLocaleTimeString();
}
async function action(path){await fetch(path,{method:'POST'});}
const start=$('#start-attack'); if(start)start.onclick=async()=>{await action('/api/attack/start?level='+($('#level')?.value||'normal'));};
const stop=$('#stop-attack'); if(stop)stop.onclick=()=>action('/api/attack/stop');
const reset=$('#reset-lab'); if(reset)reset.onclick=()=>action('/api/reset');
const mitigation=$('#toggle-mitigation'); if(mitigation)mitigation.onclick=()=>action('/api/mitigation/toggle');
const es=new EventSource('/api/events'); es.onmessage=e=>paint(JSON.parse(e.data));
es.onerror=()=>{const x=$('#connection');if(x)x.textContent='RECONNECTING…'};es.onopen=()=>{const x=$('#connection');if(x)x.textContent='LIVE CONNECTION'};
