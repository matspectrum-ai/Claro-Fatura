(() => {
  'use strict';
  const app = document.querySelector('#invoice-app');
  const toast = document.querySelector('#toast');
  const phoneFromPath = decodeURIComponent(location.pathname.replace(/^\/fatura\//, '').split('/')[0] || '');
  const payables = new Set(['em_aberto', 'vencida']);
  const statusLabels = { em_aberto:'Em aberto', em_processamento:'Em processamento', paga:'Paga', vencida:'Vencida', expirada:'Expirada', falhou:'Pagamento falhou', cancelada:'Cancelada' };
  const messages = {
    em_aberto:'Esta fatura está em aberto. Pague com PIX e a confirmação é automática.',
    em_processamento:'Recebemos seu pagamento e ele está em processamento pelo banco. A confirmação aparece aqui em alguns instantes.',
    paga:'Pagamento confirmado — esta fatura está quitada. Nada mais a fazer.',
    vencida:'Esta fatura está vencida, mas ainda pode ser paga agora com o desconto à vista.',
    expirada:'O prazo desta oferta expirou e o código PIX não é mais válido. Aproveite descontos imperdíveis entrando em contato com o atendimento.',
    falhou:'A última tentativa de pagamento não foi concluída. Gere um novo código PIX ou fale com o atendimento.',
    cancelada:'Esta fatura foi cancelada e não precisa ser paga.'
  };
  const iconPaths = {
    searchX:'<path d="m13.5 8.5-5 5"/><path d="m8.5 8.5 5 5"/><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
    shieldCheck:'<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/><path d="m9 12 2 2 4-4"/>',
    circleCheckBig:'<path d="M21.801 10A10 10 0 1 1 17 3.335"/><path d="m9 11 3 3L22 4"/>',
    circleAlert:'<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>',
    qrCode:'<rect width="5" height="5" x="3" y="3" rx="1"/><rect width="5" height="5" x="16" y="3" rx="1"/><rect width="5" height="5" x="3" y="16" rx="1"/><path d="M21 16h-3a2 2 0 0 0-2 2v3"/><path d="M21 21v.01"/><path d="M12 7v3a2 2 0 0 1-2 2H7"/><path d="M3 12h.01"/><path d="M12 3h.01"/><path d="M12 16v.01"/><path d="M16 12h1"/><path d="M21 12v.01"/><path d="M12 21v-1"/>',
    copy:'<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>',
    refreshCw:'<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>'
  };
  const icon = (name, cls='icon-sm') => `<svg class="lucide ${cls}" viewBox="0 0 24 24" aria-hidden="true">${iconPaths[name] || ''}</svg>`;

  const esc = (v) => String(v ?? '').replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const onlyDigits = (v) => String(v || '').replace(/\D/g, '').slice(0,11);
  const formatPhone = (v) => { const d=onlyDigits(v); if(d.length<=2)return d;if(d.length<=6)return`(${d.slice(0,2)}) ${d.slice(2)}`;if(d.length<=10)return`(${d.slice(0,2)}) ${d.slice(2,6)}-${d.slice(6)}`;return`(${d.slice(0,2)}) ${d.slice(2,7)}-${d.slice(7)}`; };
  const money = (v) => new Intl.NumberFormat('pt-BR',{style:'currency',currency:'BRL'}).format(Number.isFinite(Number(v)) ? Number(v) : 0);
  const date = (iso) => { if(!iso)return '—'; const [y,m,d]=String(iso).slice(0,10).split('-'); return y&&m&&d?`${d}/${m}/${y}`:'—'; };
  const notify = (message, kind='ok') => { toast.textContent=message;toast.dataset.kind=kind;toast.classList.add('show');setTimeout(()=>toast.classList.remove('show'),3000); };
  const json = async (url, options) => { const r=await fetch(url,options); const body=await r.json().catch(()=>({})); if(!r.ok) throw new Error(body.erro || 'Não foi possível concluir a solicitação.'); return body; };

  function notice(title, text) {
    app.innerHTML = `<section class="notice card"><div class="notice-icon">${icon('searchX','icon-xl')}</div><h1>${esc(title)}</h1><p>${esc(text)}</p><a class="button button-cta" href="/">Consultar outro telefone</a></section>`;
  }

  function pageShell(client, invoices) {
    app.innerHTML = `<section class="result-hero"><div class="shell narrow"><p class="eyebrow light">Resultado da consulta</p><h1>Olá, ${esc(client.nome)}!</h1><p>Telefone consultado: ${esc(formatPhone(client.telefone))}</p></div></section><main class="shell narrow invoice-main"><h2>${invoices.length?'Sua fatura':'Nenhuma fatura em aberto para este mês'}</h2><div id="invoice-list"></div><p class="privacy-note">${icon('shieldCheck')} Exibimos somente os dados vinculados ao telefone informado.</p></main>`;
    const list=document.querySelector('#invoice-list');
    if(!invoices.length){list.innerHTML=`<div class="empty card"><div class="notice-icon">${icon('searchX','icon-xl')}</div><h3>Nenhuma fatura em aberto para este mês</h3><p>Não localizamos fatura aguardando pagamento neste mês para o telefone ${esc(formatPhone(client.telefone))}. Se você já efetuou o pagamento, não há nada pendente.</p><a class="button button-cta" href="/">Consultar outro telefone</a></div>`;return;}
    invoices.forEach((invoice) => list.appendChild(invoiceCard(invoice,client)));
  }

  function badge(status){return `<span class="status status-${esc(status)}">${esc(statusLabels[status]||status)}</span>`;}
  function field(label,value,kind=''){return `<div class="field"><dt>${esc(label)}</dt><dd class="${kind}">${esc(value)}</dd></div>`;}
  function invoiceCard(invoice, client) {
    const node=document.createElement('article'); node.className='invoice-card card'; node.dataset.invoiceId=invoice.id;
    const value=Number(invoice.valor_desconto)>0?Number(invoice.valor_desconto):Number(invoice.valor_original);
    const saving=Number(invoice.valor_original)-value;
    node.innerHTML=`<div class="card-brand"><img class="brand-white-logo" src="/assets/logo-claro-white.png" alt="Logo da operadora Claro"></div>
      <div class="customer-row"><div><p class="field-label">Cliente</p><strong>${esc(client.nome)}</strong><p>${esc(formatPhone(client.telefone))}</p></div><div class="status-slot">${badge(invoice.status)}</div></div>
      <div class="amount-block"><p class="field-label">${esc(invoice.descricao)}${invoice.referencia?` · ${esc(invoice.referencia)}`:''}</p><p class="big-amount">${esc(money(value))}</p>${saving>0?`<div class="saving-row"><span class="old-price">${esc(money(invoice.valor_original))}</span><span class="saving">economia de ${esc(money(saving))}</span></div>`:''}</div>
      <dl class="field-grid">${field('Valor original',money(invoice.valor_original),'strike')}${field('Valor com desconto à vista',money(value),'highlight')}${field('Vencimento',date(invoice.vencimento))}${field('Telefone',formatPhone(client.telefone))}</dl>
      <div class="payment-area"></div>`;
    const state={status:invoice.status,pix:null,timer:null,poller:null,loading:false};
    renderPayment(node,state,invoice,client);
    if(payables.has(state.status)) generatePix(node,state,invoice,false);
    if(state.status==='em_processamento') startPolling(node,state,invoice);
    return node;
  }

  function renderPayment(node,state,invoice,client){
    const area=node.querySelector('.payment-area');
    node.querySelector('.status-slot').innerHTML=badge(state.status);
    const msg=messages[state.status]||'Não foi possível determinar a situação desta fatura.';
    if(state.status==='paga'){
      stopTimers(state); area.innerHTML=`<div class="state-box success">${icon('circleCheckBig','icon-md')}<strong>${esc(msg)}</strong></div>`;return;
    }
    if(state.status==='em_processamento'){
      area.innerHTML=`<div class="state-box"><span class="spinner small"></span><strong>${esc(msg)}</strong></div>`;startPolling(node,state,invoice);return;
    }
    if(!payables.has(state.status)){
      stopTimers(state);area.innerHTML=`<div class="state-box error">${icon('circleAlert','icon-md')}<strong>${esc(statusLabels[state.status]||state.status)}</strong><p>${esc(msg)}</p></div>`;return;
    }
    const pix=state.pix;
    area.innerHTML=`<p class="status-message">${esc(msg)}</p><div class="pix-box"><p class="pix-title">${icon('qrCode')}<strong>Pague com PIX</strong></p><p class="small-muted">Escaneie o QR Code ou copie o código no app do seu banco.</p><div class="qr-frame"><div class="qr-content">${state.loading?'<span class="spinner"></span>':'<p class="small-muted">Gerando código PIX…</p>'}</div></div><p class="copy-code" hidden></p><button class="button button-cta button-large copy-pix" disabled>${icon('copy','icon-md')} Copiar código PIX</button><button class="button button-outline new-pix">${icon('refreshCw')} Gerar novo PIX</button><p class="expiry" hidden></p><p class="waiting"><span class="spinner tiny"></span> Aguardando confirmação do pagamento…</p></div><p class="privacy-note centered">${icon('shieldCheck')} A confirmação é automática assim que o banco liquidar o PIX.</p>`;
    area.querySelector('.new-pix').addEventListener('click',()=>generatePix(node,state,invoice,true));
    area.querySelector('.copy-pix').addEventListener('click',async()=>{if(!state.pix?.copia_cola)return;try{await navigator.clipboard.writeText(state.pix.copia_cola);notify('Código PIX copiado! Cole no app do seu banco.');}catch{notify('Não foi possível copiar. Selecione o código manualmente.','error');}});
    if(pix) updatePixUI(node,state,invoice);
  }

  function renderQR(container, code) {
    try {
      if(typeof window.qrcode !== 'function') throw new Error('QR encoder indisponível');
      const qr=window.qrcode(0,'M'); qr.addData(code); qr.make();
      container.innerHTML=qr.createSvgTag({cellSize:4,margin:4,scalable:true,alt:'QR Code para pagamento PIX da fatura'});
      const svg=container.querySelector('svg'); if(svg){svg.setAttribute('width','100%');svg.setAttribute('height','100%');}
    } catch { container.innerHTML='<p class="small-muted">Não foi possível gerar o QR Code agora.</p>'; }
  }

  function updatePixUI(node,state,invoice){
    const area=node.querySelector('.payment-area'); const pix=state.pix;if(!pix)return;
    const qr=area.querySelector('.qr-content');
    if(pix.disponivel && pix.copia_cola){ renderQR(qr,pix.copia_cola); const text=area.querySelector('.copy-code');text.hidden=false;text.textContent=pix.copia_cola;area.querySelector('.copy-pix').disabled=false;startCountdown(area,state,pix.expira_em);startPolling(node,state,invoice); }
    else { qr.innerHTML=`<p class="small-muted">${esc(pix.mensagem||'Pagamento indisponível no momento. Tente novamente em alguns minutos.')}</p>`; }
  }

  async function generatePix(node,state,invoice,force){
    if(state.loading)return;state.loading=true; if(force)state.pix=null;renderPayment(node,state,invoice,{});
    try{
      const data=await json(`/api/v1/faturas/${encodeURIComponent(invoice.id)}/pix`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({request_key:crypto.randomUUID(),forcar:Boolean(force)})});
      state.pix=data; if(data.status==='paga')state.status='paga'; if(force)notify(data.disponivel?'Novo código PIX gerado.':(data.mensagem||'Pagamento indisponível no momento.'),data.disponivel?'ok':'error');
    }catch(err){state.pix={disponivel:false,mensagem:'Pagamento indisponível no momento. Tente novamente em alguns minutos.'};if(force)notify('Não foi possível gerar um novo PIX agora.','error');}
    finally{state.loading=false;renderPayment(node,state,invoice,{});}
  }

  function startCountdown(area,state,iso){
    if(state.timer){clearInterval(state.timer);state.timer=null;} const el=area.querySelector('.expiry');if(!iso||!el)return;
    const target=new Date(iso).getTime(); const tick=()=>{const left=Math.max(0,Math.floor((target-Date.now())/1000));el.hidden=false;const mm=String(Math.floor(left/60)).padStart(2,'0'),ss=String(left%60).padStart(2,'0');el.textContent=left===0?'Este código PIX expirou. Toque em “Gerar novo PIX”.':`Este código PIX expira em ${mm}:${ss}`;};tick();state.timer=setInterval(tick,1000);
  }
  function startPolling(node,state,invoice){if(state.poller||state.status==='paga')return;state.poller=setInterval(async()=>{try{const r=await json(`/api/v1/faturas/${encodeURIComponent(invoice.id)}/status`,{method:'POST'});if(r.status&&r.status!==state.status){state.status=r.status;renderPayment(node,state,invoice,{});}if(r.status==='paga'){stopTimers(state);renderPayment(node,state,invoice,{})}}catch{}},5000);}
  function stopTimers(state){if(state.timer){clearInterval(state.timer);state.timer=null;}if(state.poller){clearInterval(state.poller);state.poller=null;}}

  async function load(){
    const d=onlyDigits(phoneFromPath);if(d.length<10||d.length>11){notice('Não foi possível consultar','Verifique o telefone informado e tente novamente em instantes.');return;}
    try{const result=await json(`/api/v1/faturas?telefone=${encodeURIComponent(d)}`);if(!result.encontrado||!result.cliente){notice('Telefone não localizado',`Não encontramos cadastro para ${formatPhone(d)}. Confira o número digitado ou fale com o atendimento.`);return;}pageShell(result.cliente,result.faturas||[]);}catch{notice('Não foi possível consultar','Verifique o telefone informado e tente novamente em instantes.');}
  }
  load();
})();
