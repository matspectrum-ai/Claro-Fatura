(() => {
  'use strict';

  const state = { gateways: [], routing: null, webhooks: new Map(), busy: false };
  const $ = (id) => document.getElementById(id);
  const list = $('gateway-list');
  const dialog = $('gateway-dialog');
  const form = $('gateway-form');
  const strategy = $('routing-strategy');
  const fixedWrap = $('fixed-gateway-wrap');
  const fixedGateway = $('fixed-gateway');
  const newPix = $('new-pix-per-access');
  const routingDescription = $('routing-description');
  const toastEl = $('toast');
  let toastTimer = 0;

  function toast(message, error = false) {
    clearTimeout(toastTimer);
    toastEl.textContent = message;
    toastEl.classList.toggle('error', error);
    toastEl.classList.add('show');
    toastTimer = setTimeout(() => toastEl.classList.remove('show'), 2600);
  }

  async function api(url, options = {}) {
    const headers = new Headers(options.headers || {});
    if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
    const response = await fetch(url, { credentials: 'same-origin', ...options, headers });
    if (response.status === 401 || response.status === 403) {
      location.replace('/auth');
      throw new Error('Sessão expirada.');
    }
    const text = await response.text();
    let data = null;
    if (text) {
      try { data = JSON.parse(text); } catch { data = text; }
    }
    if (!response.ok) throw new Error(data?.erro || 'Não foi possível concluir a operação.');
    return data;
  }

  function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>'"]/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[ch]));
  }

  function formatDate(value) {
    if (!value) return '';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString('pt-BR');
  }

  function webhookFor(gateway) {
    return gateway.webhook_url || `${location.origin}/api/public/webhooks/${gateway.slug}`;
  }

  function renderRouting() {
    const routing = state.routing || { estrategia: 'prioridade', gateway_fixa: null, novo_pix_por_acesso: true };
    strategy.value = routing.estrategia || 'prioridade';
    newPix.checked = routing.novo_pix_por_acesso ?? true;
    fixedWrap.hidden = strategy.value !== 'fixa';

    const current = routing.gateway_fixa || '';
    fixedGateway.innerHTML = '<option value="">Escolha a gateway</option>' + state.gateways.map((g) => `<option value="${escapeHTML(g.id)}">${escapeHTML(g.rotulo)}</option>`).join('');
    fixedGateway.value = current;

    const active = state.gateways.filter((g) => g.ativo).length;
    routingDescription.textContent = active === 0 ? 'Nenhuma gateway ativa — nenhum PIX será gerado.' : `${active} gateway(s) ativa(s).`;
  }

  function renderGateways() {
    if (!state.gateways.length) {
      list.innerHTML = '<div class="admin-empty">Nenhuma gateway cadastrada.</div>';
      return;
    }
    list.innerHTML = state.gateways.map((g) => {
      const hook = state.webhooks.get(g.slug);
      const webhook = webhookFor(g);
      const limit = g.limite_diario ? `<span class="gateway-badge">Limite ${escapeHTML(g.limite_diario)}/dia</span>` : '';
      const creds = g.configurado
        ? '<p class="gateway-line ok"><span class="gateway-icon">●</span><span>Credenciais configuradas</span></p>'
        : '<p class="gateway-line warn"><span class="gateway-icon">▲</span><span>Credenciais pendentes — a gateway será ignorada</span></p>';
      const hookLine = hook?.ultimo_em
        ? `<p class="gateway-line ok"><span class="gateway-icon">●</span><span>Último webhook em ${escapeHTML(formatDate(hook.ultimo_em))} · ${escapeHTML(hook.total_24h)} nas últimas 24h</span></p>`
        : '<p class="gateway-line warn"><span class="gateway-icon">▲</span><span>Nenhum webhook recebido ainda desta gateway</span></p>';
      return `<article class="gateway-card" data-id="${escapeHTML(g.id)}">
        <div class="gateway-main">
          <div class="gateway-title"><strong>${escapeHTML(g.rotulo)}</strong><span class="gateway-badge ${g.ativo ? 'active' : 'inactive'}">${g.ativo ? 'Ativo' : 'Inativo'}</span><span class="gateway-badge">Prioridade ${escapeHTML(g.prioridade)}</span><span class="gateway-badge">${g.ambiente === 'teste' ? 'Teste' : 'Produção'}</span>${limit}</div>
          ${creds}${hookLine}
          <p class="gateway-line gateway-webhook"><span>Webhook: ${escapeHTML(webhook)} <button class="copy-link" type="button" data-action="copy" data-value="${escapeHTML(webhook)}">⧉ copiar</button></span></p>
        </div>
        <div class="gateway-actions">
          <button class="admin-btn outline compact" type="button" data-action="edit">✎ Editar</button>
          <button class="admin-btn outline compact" type="button" data-action="only">⚡ Usar somente esta</button>
          <button class="admin-btn outline compact danger-icon" type="button" data-action="remove" aria-label="Remover ${escapeHTML(g.rotulo)}">⌫</button>
          <label class="gateway-switch"><input type="checkbox" data-action="active" ${g.ativo ? 'checked' : ''}><span>${g.ativo ? 'Ativa' : 'Inativa'}</span></label>
        </div>
      </article>`;
    }).join('');
  }

  async function loadGateways() {
    const rows = await api('/api/admin/gateways');
    state.gateways = Array.isArray(rows) ? rows : [];
    renderRouting();
    renderGateways();
  }

  async function loadRouting() {
    state.routing = await api('/api/admin/roteamento');
    renderRouting();
  }

  async function loadWebhookSummary() {
    const rows = await api('/api/admin/gateways/webhooks-resumo');
    state.webhooks = new Map((Array.isArray(rows) ? rows : []).map((row) => [row.gateway_slug, row]));
    renderGateways();
  }

  async function reloadAll() {
    await Promise.all([loadGateways(), loadRouting(), loadWebhookSummary()]);
  }

  async function saveRouting(patch = {}) {
    const current = state.routing || { estrategia: 'prioridade', gateway_fixa: null, novo_pix_por_acesso: true };
    const next = {
      estrategia: patch.estrategia ?? current.estrategia ?? 'prioridade',
      gateway_fixa: patch.gateway_fixa !== undefined ? patch.gateway_fixa : current.gateway_fixa,
      novo_pix_por_acesso: patch.novo_pix_por_acesso !== undefined ? patch.novo_pix_por_acesso : (current.novo_pix_por_acesso ?? true),
    };
    if (next.estrategia !== 'fixa') next.gateway_fixa = null;
    await api('/api/admin/roteamento', { method: 'POST', body: JSON.stringify(next) });
    state.routing = next;
    renderRouting();
    toast('Estratégia atualizada.');
  }

  function resetForm(gateway = null) {
    $('gateway-id').value = gateway?.id || '';
    $('gateway-label').value = gateway?.rotulo || '';
    $('gateway-slug').value = gateway?.slug || '';
    $('gateway-adapter').value = gateway?.adapter || 'generico';
    $('gateway-environment').value = gateway?.ambiente === 'teste' ? 'teste' : 'producao';
    $('gateway-api-url').value = gateway?.api_url || '';
    $('gateway-priority').value = String(gateway?.prioridade ?? 50);
    $('gateway-daily-limit').value = gateway?.limite_diario == null ? '' : String(gateway.limite_diario);
    $('gateway-webhook-url').value = gateway?.webhook_url || '';
    $('gateway-secret-names').value = Array.isArray(gateway?.secret_names) ? gateway.secret_names.join(', ') : '';
    $('gateway-observations').value = gateway?.observacoes || '';
    $('gateway-active').checked = gateway?.ativo ?? false;
    $('gateway-dialog-title').textContent = gateway ? 'Editar gateway' : 'Nova gateway';
  }

  function openForm(gateway = null) {
    resetForm(gateway);
    if (typeof dialog.showModal === 'function') dialog.showModal();
    else dialog.setAttribute('open', '');
  }

  function closeForm() {
    if (typeof dialog.close === 'function') dialog.close();
    else dialog.removeAttribute('open');
  }

  function nullable(value) {
    const text = String(value || '').trim();
    return text ? text : null;
  }

  async function saveGateway(event) {
    event.preventDefault();
    if (state.busy) return;
    state.busy = true;
    $('gateway-save').disabled = true;
    try {
      const id = $('gateway-id').value.trim();
      const dailyRaw = $('gateway-daily-limit').value.trim();
      const payload = {
        ...(id ? { id } : {}),
        slug: $('gateway-slug').value.trim().toLowerCase(),
        rotulo: $('gateway-label').value.trim(),
        adapter: $('gateway-adapter').value.trim(),
        api_url: nullable($('gateway-api-url').value),
        ambiente: $('gateway-environment').value,
        prioridade: Number($('gateway-priority').value) || 50,
        limite_diario: dailyRaw ? Number(dailyRaw) : null,
        webhook_url: nullable($('gateway-webhook-url').value),
        secret_names: $('gateway-secret-names').value.split(',').map((name) => name.trim().toUpperCase()).filter(Boolean),
        observacoes: nullable($('gateway-observations').value),
        ativo: $('gateway-active').checked,
      };
      await api('/api/admin/gateways', { method: 'POST', body: JSON.stringify(payload) });
      closeForm();
      await reloadAll();
      toast('Gateway salvo.');
    } catch (error) {
      toast(error.message || 'Não foi possível salvar.', true);
    } finally {
      state.busy = false;
      $('gateway-save').disabled = false;
    }
  }

  async function patchActive(gateway, active, input) {
    input.disabled = true;
    try {
      await api(`/api/admin/gateways/${encodeURIComponent(gateway.id)}`, { method: 'PATCH', body: JSON.stringify({ ativo: active }) });
      await loadGateways();
      toast('Configuração salva.');
    } catch (error) {
      input.checked = !active;
      toast(error.message || 'Não foi possível salvar.', true);
    } finally { input.disabled = false; }
  }

  async function useOnly(gateway) {
    try {
      await api(`/api/admin/gateways/${encodeURIComponent(gateway.id)}/somente`, { method: 'POST' });
      await Promise.all([loadGateways(), loadRouting()]);
      toast('Gateway definido como único ativo.');
    } catch (error) { toast(error.message || 'Não foi possível ativar.', true); }
  }

  async function removeGateway(gateway) {
    if (!confirm(`Remover a gateway ${gateway.rotulo}?`)) return;
    try {
      await api(`/api/admin/gateways/${encodeURIComponent(gateway.id)}`, { method: 'DELETE' });
      await reloadAll();
      toast('Gateway removido.');
    } catch (error) { toast(error.message || 'Não foi possível remover.', true); }
  }

  list.addEventListener('click', async (event) => {
    const target = event.target.closest('[data-action]');
    if (!target) return;
    const card = target.closest('[data-id]');
    const gateway = state.gateways.find((item) => item.id === card?.dataset.id);
    if (!gateway) return;
    const action = target.dataset.action;
    if (action === 'edit') openForm(gateway);
    if (action === 'only') await useOnly(gateway);
    if (action === 'remove') await removeGateway(gateway);
    if (action === 'copy') {
      try { await navigator.clipboard.writeText(target.dataset.value || webhookFor(gateway)); toast('Endereço do webhook copiado.'); }
      catch { toast('Não foi possível copiar.', true); }
    }
  });

  list.addEventListener('change', async (event) => {
    const input = event.target.closest('input[data-action="active"]');
    if (!input) return;
    const card = input.closest('[data-id]');
    const gateway = state.gateways.find((item) => item.id === card?.dataset.id);
    if (gateway) await patchActive(gateway, input.checked, input);
  });

  strategy.addEventListener('change', async () => {
    fixedWrap.hidden = strategy.value !== 'fixa';
    try { await saveRouting({ estrategia: strategy.value, gateway_fixa: strategy.value === 'fixa' ? (state.routing?.gateway_fixa || null) : null }); }
    catch (error) { toast(error.message || 'Não foi possível salvar a estratégia.', true); await loadRouting(); }
  });

  fixedGateway.addEventListener('change', async () => {
    try { await saveRouting({ estrategia: 'fixa', gateway_fixa: fixedGateway.value || null }); }
    catch (error) { toast(error.message || 'Não foi possível salvar a estratégia.', true); await loadRouting(); }
  });

  newPix.addEventListener('change', async () => {
    try { await saveRouting({ novo_pix_por_acesso: newPix.checked }); }
    catch (error) { toast(error.message || 'Não foi possível salvar a estratégia.', true); await loadRouting(); }
  });

  $('new-gateway').addEventListener('click', () => openForm());
  $('gateway-cancel').addEventListener('click', closeForm);
  form.addEventListener('submit', saveGateway);
  $('logout').addEventListener('click', async () => { try { await api('/api/auth/logout', { method: 'POST' }); } finally { location.replace('/auth'); } });

  async function boot() {
    try {
      await api('/api/auth/me');
      await reloadAll();
      setInterval(() => loadGateways().catch(() => {}), 10000);
      setInterval(() => loadWebhookSummary().catch(() => {}), 30000);
    } catch (error) {
      if (location.pathname !== '/auth') console.error(error);
    }
  }

  boot();
})();
