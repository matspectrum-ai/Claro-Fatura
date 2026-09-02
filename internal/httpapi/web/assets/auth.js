(() => {
  'use strict';
  const page=document.body.dataset.authPage;
  const message=document.querySelector('#auth-message');
  const show=(text,kind='error')=>{if(!message)return;message.textContent=text;message.dataset.kind=kind;message.hidden=false};
  const busy=(form,on)=>{form.querySelectorAll('input,button').forEach((el)=>el.disabled=on);const b=form.querySelector('button[type=submit]');if(b){if(!b.dataset.label)b.dataset.label=b.textContent;b.textContent=on?'Aguarde…':b.dataset.label}};
  const api=async(url,options={})=>{const res=await fetch(url,{...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});const body=await res.json().catch(()=>({}));if(!res.ok)throw new Error(body.erro||'Não foi possível concluir a solicitação.');return body};

  if(page==='login'){
    fetch('/api/auth/me').then(r=>{if(r.ok)location.replace('/admin')}).catch(()=>{});
    const tabs=document.querySelectorAll('.auth-tab'),login=document.querySelector('#login-form'),signup=document.querySelector('#signup-form');
    tabs.forEach(t=>t.addEventListener('click',()=>{tabs.forEach(x=>x.classList.toggle('active',x===t));const isLogin=t.dataset.tab==='login';login.hidden=!isLogin;signup.hidden=isLogin;message.hidden=true}));
    login.addEventListener('submit',async e=>{e.preventDefault();busy(login,true);message.hidden=true;const f=new FormData(login);try{await api('/api/auth/login',{method:'POST',body:JSON.stringify({email:f.get('email'),senha:f.get('senha')})});location.replace('/admin')}catch(err){show(err.message)}finally{busy(login,false)}});
    signup.addEventListener('submit',async e=>{e.preventDefault();busy(signup,true);message.hidden=true;const f=new FormData(signup);try{const out=await api('/api/auth/signup',{method:'POST',body:JSON.stringify({nome:f.get('nome'),email:f.get('email'),senha:f.get('senha')})});if(out.confirmacao_email){show('Cadastro criado. Confirme o e-mail enviado para ativar o acesso.','ok')}else location.replace('/admin')}catch(err){show(err.message)}finally{busy(signup,false)}});
  }

  if(page==='forgot'){
    const form=document.querySelector('#forgot-form'),success=document.querySelector('#forgot-success');form.addEventListener('submit',async e=>{e.preventDefault();busy(form,true);message.hidden=true;const f=new FormData(form);try{await api('/api/auth/recover',{method:'POST',body:JSON.stringify({email:f.get('email')})});document.querySelector('#forgot-email').textContent=f.get('email');form.hidden=true;success.hidden=false}catch(err){show(err.message)}finally{busy(form,false)}});
  }

  if(page==='reset'){
    const form=document.querySelector('#reset-form'),invalid=document.querySelector('#reset-invalid');
    const toggle=document.querySelector('.password-toggle');
    if(toggle){toggle.addEventListener('click',()=>{const input=form.querySelector('input[name="senha"]');const showing=input.type==='text';input.type=showing?'password':'text';toggle.setAttribute('aria-pressed',String(!showing));toggle.setAttribute('aria-label',showing?'Mostrar senha':'Ocultar senha');toggle.querySelector('.eye-on').hidden=!showing;toggle.querySelector('.eye-off').hidden=showing})}
    const boot=async()=>{const p=new URLSearchParams(location.hash.replace(/^#/,''));const access=p.get('access_token'),refresh=p.get('refresh_token'),type=p.get('type');if(type==='recovery'&&access){try{await api('/api/auth/recovery-session',{method:'POST',body:JSON.stringify({access_token:access,refresh_token:refresh||'',expires_in:Number(p.get('expires_in'))||3600})});history.replaceState(null,'',location.pathname);form.hidden=false;return}catch{}}const me=await fetch('/api/auth/me').catch(()=>null);if(me&&me.ok){form.hidden=false}else invalid.hidden=false};boot();
    form.addEventListener('submit',async e=>{e.preventDefault();const f=new FormData(form),a=String(f.get('senha')||''),b=String(f.get('confirmar')||'');if(a.length<6){show('A senha deve ter pelo menos 6 caracteres.');return}if(a!==b){show('Digite a mesma senha nos dois campos.');return}busy(form,true);message.hidden=true;try{await api('/api/auth/password',{method:'POST',body:JSON.stringify({senha:a})});show('Senha redefinida. Faça login para continuar.','ok');setTimeout(()=>location.replace('/auth'),900)}catch(err){show(err.message)}finally{busy(form,false)}});
  }
})();
