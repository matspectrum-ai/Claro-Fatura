(() => {
  'use strict';
  const $ = (s) => document.querySelector(s);
  const form = $('#consulta-form');
  const phone = $('#telefone');
  const terms = $('#aceite');
  const submit = $('#consultar');
  const toast = $('#toast');

  const digits = (value) => String(value || '').replace(/\D/g, '').slice(0, 11);
  const formatPhone = (value) => {
    const d = digits(value);
    if (d.length <= 2) return d;
    if (d.length <= 6) return `(${d.slice(0, 2)}) ${d.slice(2)}`;
    if (d.length <= 10) return `(${d.slice(0, 2)}) ${d.slice(2, 6)}-${d.slice(6)}`;
    return `(${d.slice(0, 2)}) ${d.slice(2, 7)}-${d.slice(7)}`;
  };
  const valid = () => {
    const n = digits(phone.value).length;
    submit.disabled = !(n >= 10 && n <= 11 && terms.checked);
  };
  const notify = (message) => {
    toast.textContent = message;
    toast.classList.add('show');
    setTimeout(() => toast.classList.remove('show'), 2800);
  };

  phone.addEventListener('input', () => { phone.value = formatPhone(phone.value); valid(); });
  terms.addEventListener('change', valid);
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    const value = digits(phone.value);
    if (value.length < 10 || value.length > 11) { notify('Informe um telefone válido.'); return; }
    if (!terms.checked) return;
    submit.disabled = true;
    submit.textContent = 'Consultando…';
    location.href = `/fatura/${encodeURIComponent(value)}`;
  });

  // A visita pública é contabilizada de forma silenciosa, como na implementação original.
  fetch('/api/v1/acessos', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{"pagina":"/"}', keepalive: true }).catch(() => {});
})();
