/* Fixture exclusively for preview.html. Vite's production build uses index.html, not this file. */
(() => {
  if (!['localhost', '127.0.0.1'].includes(window.location.hostname)) throw new Error('La vista previa solo funciona en este equipo.');
  const nativeFetch = window.fetch.bind(window);
  const now = new Date();
  const iso = offset => new Date(now.getTime() - offset * 60000).toISOString();
  const item = (kind, name, displayName, spec) => ({ apiVersion:'modelcairn.io/v1alpha1', kind, state:'present', metadata:{ name, displayName, resourceVersion:1 }, spec });
  const ref = name => ({ name });
  const resources = {
    providers:[item('Provider','google','Google AI Studio',{})],
    'provider-accounts':[item('ProviderAccount','google-demo','Cuenta de ejemplo',{ providerRef:ref('google') })],
    'provider-connections':[item('ProviderConnection','google-api','Google API',{ providerRef:ref('google'), baseUrl:'https://generativelanguage.googleapis.com/v1beta/openai', enabled:true })],
    credentials:[item('Credential','google-key','Clave de ejemplo',{ providerAccountRef:ref('google-demo'), secretRef:ref('clave-demo'), egressRef:ref('salida-local'), enabled:true })],
    egresses:[item('Egress','salida-local','Salida local',{ enabled:true })],
    models:[item('Model','gemini-flash','Gemini Flash',{ connectionRef:ref('google-api'), providerModelId:'gemini-flash', capabilities:['text','stream','tools'], enabled:true, pricing:{ currency:'USD', inputPerMillion:0.15, outputPerMillion:0.6 } })],
    destinations:[item('Destination','destino-demo','Gemini Flash · ejemplo',{ modelRef:ref('gemini-flash'), credentialRef:ref('google-key'), enabled:true })],
    strategies:[item('Strategy','estrategia-demo','Respaldo de ejemplo',{ destinations:[ref('destino-demo')], enabled:true })],
    routes:[item('Route','assistant','assistant',{ modelAlias:'assistant', strategyRef:ref('estrategia-demo'), enabled:true })],
    'agent-tokens':[item('AgentToken','cliente-demo','Cliente de ejemplo',{ routeRefs:[ref('assistant')] })]
  };
  const requests = [
    { id:'preview-1', requestedAlias:'assistant', startedAt:iso(2), completedAt:iso(2), outcome:'success', httpStatus:200, durationMs:780, durationMillis:780, inputTokens:750, outputTokens:210, attempts:1 },
    { id:'preview-2', requestedAlias:'assistant', startedAt:iso(11), completedAt:iso(11), outcome:'success', httpStatus:200, durationMs:1240, durationMillis:1240, inputTokens:810, outputTokens:180, attempts:2 },
    { id:'preview-3', requestedAlias:'assistant', startedAt:iso(18), completedAt:iso(18), outcome:'error', httpStatus:503, durationMs:620, durationMillis:620, inputTokens:0, outputTokens:0, attempts:1 }
  ];
  const daily = Array.from({ length:30 }, (_, index) => {
    const date = new Date(now); date.setUTCDate(date.getUTCDate() - 29 + index);
    const count = 8 + ((index * 7) % 17);
    return { date:date.toISOString().slice(0,10), requests:count, success:count - (index % 9 === 0 ? 1 : 0), inputTokens:count * 730, outputTokens:count * 205 };
  });
  const settingsSpec = { publicOrigin:'http://127.0.0.1:8080', listen:'127.0.0.1:8080', transport:'loopback-http', trustedProxyCidrs:[], tlsCertificatePath:'', tlsPrivateKeyPath:'', idleSeconds:1800, absoluteSeconds:43200, globalAttemptsPerMinute:30, globalBurst:5, clientAttemptsPerMinute:5, clientBurst:3, maxClientEntries:1024, clientIdleSeconds:900, failedLoginRetentionSeconds:86400, argonMemoryKiB:19456, argonIterations:2 };
  const settingsDocument = { apiVersion:'modelcairn.io/v1alpha1', kind:'AdminSettings', resourceVersion:1, spec:settingsSpec };
  const json = (body, status=200) => Promise.resolve(new Response(JSON.stringify(body), { status, headers:{ 'Content-Type':'application/json' } }));
  window.fetch = (input, init={}) => {
    const url = new URL(typeof input === 'string' ? input : input.url, window.location.href);
    if (url.pathname !== '/readyz' && !url.pathname.startsWith('/api/v1/admin/')) return nativeFetch(input, init);
    if ((init.method || 'GET').toUpperCase() !== 'GET') return json({ error:{ code:'preview_read_only' } }, 403);
    const path = url.pathname.replace(/^\/api\/v1\/admin/, '');
    if (url.pathname === '/readyz') return json({ status:'ready', components:{} });
    if (path === '/session/me') return json({ admin:{ id:'preview-admin', username:'Vista previa' }, csrfToken:'preview-only', expiresAt:iso(-60) });
    if (path === '/overview') return json({ resourceCounts:{ Provider:1, Model:1, Route:1, Destination:1 }, requests24h:{ total:1284, success:1240, error:44 }, activeCooldowns:1, recentRequests:requests, generatedAt:now.toISOString() });
    if (path === '/metrics') return json({ requests:daily.reduce((sum, row) => sum + row.requests,0), inputTokens:daily.reduce((sum,row) => sum + row.inputTokens,0), outputTokens:daily.reduce((sum,row) => sum + row.outputTokens,0), usageKnownRequests:daily.reduce((sum,row) => sum + row.requests,0), daily, models:[{ name:'gemini-flash', requests:412, inputTokens:302000, outputTokens:84500, avgLatencyMillis:980, outputTokensPerSecond:28.4 }], generatedAt:now.toISOString() });
    if (path.startsWith('/resources/')) return json({ items:resources[decodeURIComponent(path.split('/')[2])] || [], nextCursor:null });
    if (path === '/secrets') return json({ items:[{ name:'clave-demo', resourceVersion:1, fingerprint:'ejemplo-no-es-una-clave' }], nextCursor:null });
    if (path === '/settings') return json({ desired:settingsDocument, effective:settingsDocument, restartRequired:false });
    if (path === '/agent-tokens/cliente-demo/status') return json({ tokenStatus:{ state:'active', prefix:'mc_preview_' } });
    if (path === '/requests') {
      const filtered = requests.filter(row => (!url.searchParams.get('outcome') || row.outcome === url.searchParams.get('outcome')) && (!url.searchParams.get('alias') || row.requestedAlias.includes(url.searchParams.get('alias'))));
      return json({ items:filtered, nextCursor:null });
    }
    if (path.startsWith('/requests/') && path.endsWith('/attempts')) return json({ items:[{ sequence:1, outcome:'success', providerStatus:200, retryable:false }] });
    return json({ error:{ code:'preview_endpoint_unavailable' } }, 404);
  };
  window.__MODELCAIRN_PREVIEW_READY__ = true;
})();
