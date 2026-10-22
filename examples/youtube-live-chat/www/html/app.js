const eventsEl = document.getElementById('events');
const videoIdEl = document.getElementById('videoId');
const apiKeyEl = document.getElementById('apiKey');
const externalWsUrlEl = document.getElementById('externalWsUrl');

function appendEvent(line) {
  eventsEl.textContent = `${line}\n${eventsEl.textContent}`;
}

async function loadConfig() {
  try {
    const resp = await fetch('/api/config');
    const cfg = await resp.json();
    if (cfg.videoId) videoIdEl.value = cfg.videoId;
    if (cfg.externalWsUrl) externalWsUrlEl.value = cfg.externalWsUrl;
  } catch (err) {
    appendEvent(`[config-error] ${err.message}`);
  }
}

async function postJSON(url, data) {
  const resp = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  const body = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new Error(body.error || `HTTP ${resp.status}`);
  }
  return body;
}

document.getElementById('startBtn').addEventListener('click', async () => {
  try {
    await postJSON('/api/start', {
      videoId: videoIdEl.value.trim(),
      apiKey: apiKeyEl.value.trim()
    });
    appendEvent('[status] 已送出開始請求');
  } catch (err) {
    appendEvent(`[start-error] ${err.message}`);
  }
});

document.getElementById('stopBtn').addEventListener('click', async () => {
  try {
    await postJSON('/api/stop', {});
    appendEvent('[status] 已送出停止請求');
  } catch (err) {
    appendEvent(`[stop-error] ${err.message}`);
  }
});

function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${proto}://${location.host}/ws`);

  ws.onopen = () => appendEvent('[ws] connected');
  ws.onclose = () => {
    appendEvent('[ws] disconnected, retry in 2s');
    setTimeout(connectWS, 2000);
  };
  ws.onerror = () => appendEvent('[ws] error');
  ws.onmessage = (event) => {
    appendEvent(event.data);
  };
}

loadConfig();
connectWS();
