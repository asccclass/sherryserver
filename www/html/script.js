import { HeadAudio } from "/headaudio/headaudio.min.mjs";
import { ASSISTANT_TEXT_FILTER_RULES } from "/assistant-text-filter-rules.js";

const state = {
  ws: null,
  connectionState: "disconnected",
  sessionId: "",
  avatars: [],
  selectedAvatarId: "",
  audioContext: null,
  captureNode: null,
  captureSource: null,
  mediaStream: null,
  playerContext: null,
  nextPlaybackTime: 0,
  micOn: false,
  inputTranscriptNode: null,
  outputTranscriptNode: null,
  inputTranscriptText: "",
  inputTurnActive: false,
  outputTranscriptText: "",
  outputInterrupted: false,
  mouthCloseTimer: null,
  playbackDelayNode: null,
  visemeAnalyzer: null,
  visemeReady: null,
  visemeRAF: 0,
  visemeLastTick: 0,
  visemeEnabled: false,
  fallbackAnalyser: null,
  activeAssistantBubble: null,
  livePaneHideTimers: new Map(),
};

const el = {};

function init() {
  ["avatarSelect", "avatarSummary", "model", "systemInstruction", "voice", "temperature", "inputTranscription", "outputTranscription", "connectBtn", "disconnectBtn", "status", "sessionMeta", "feed", "statusFeed", "llmPromptPane", "llmPrompt", "inputTranscriptPane", "inputTranscript", "outputTranscriptPane", "outputTranscript", "textInput", "sendBtn", "micBtn", "debug", "connectionLight", "connectionLabel", "heroName", "heroSubtitle"].forEach((id) => {
    el[id] = document.getElementById(id);
  });

  el.connectBtn.addEventListener("click", connect);
  el.disconnectBtn.addEventListener("click", disconnect);
  el.sendBtn.addEventListener("click", sendText);
  el.textInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter") sendText();
  });
  el.micBtn.addEventListener("click", toggleMic);
  el.avatarSelect.addEventListener("change", onAvatarChange);

  setConnectionState("connecting");
  pushStatus("頁面已載入，正在準備自動連線。");
  renderSessionMeta();
  loadAvatars().catch((error) => {
    console.error(error);
    setConnectionState("disconnected");
    pushStatus(`載入角色失敗：${error.message}`);
  });
}

function renderSessionMeta() {
  el.sessionMeta.textContent = state.sessionId
    ? `工作階段 ID：${state.sessionId}`
    : "工作階段 ID：尚未建立";
}

function refreshStatusFeedVisibility() {
  if (!el.statusFeed) return;
  el.statusFeed.classList.toggle("empty", el.statusFeed.children.length === 0);
}

function setPaneVisibility(node, visible) {
  if (!node) return;

  const existingTimer = state.livePaneHideTimers.get(node.id);
  if (existingTimer) {
    clearTimeout(existingTimer);
    state.livePaneHideTimers.delete(node.id);
  }

  if (visible) {
    node.hidden = false;
    requestAnimationFrame(() => {
      node.classList.add("is-visible");
    });
    return;
  }

  node.classList.remove("is-visible");
  const hideTimer = setTimeout(() => {
    node.hidden = true;
    state.livePaneHideTimers.delete(node.id);
  }, 180);
  state.livePaneHideTimers.set(node.id, hideTimer);
}

function refreshLivePaneVisibility() {
  const micActive = state.micOn;
  const hasPrompt = !el.llmPrompt.classList.contains("empty");
  const hasInputTranscript = !el.inputTranscript.classList.contains("empty");
  const hasOutputTranscript = !el.outputTranscript.classList.contains("empty");

  setPaneVisibility(el.llmPromptPane, micActive && hasPrompt);
  setPaneVisibility(el.inputTranscriptPane, micActive && hasInputTranscript);
  setPaneVisibility(el.outputTranscriptPane, micActive && hasOutputTranscript);
}

async function ensureVisemeAnalyzer() {
  if (state.visemeReady) {
    await state.visemeReady;
    return;
  }

  state.visemeReady = (async () => {
    if (!state.playerContext) {
      state.playerContext = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 24000 });
      state.nextPlaybackTime = 0;
      state.fallbackAnalyser = state.playerContext.createAnalyser();
      state.fallbackAnalyser.fftSize = 512;
      state.fallbackAnalyser.connect(state.playerContext.destination);
    }

    await state.playerContext.audioWorklet.addModule("/headaudio/headworklet.min.mjs");

    const analyzer = new HeadAudio(state.playerContext, {
      processorOptions: {
        visemeEventsEnabled: true,
      },
      parameterData: {
        vadGateActiveDb: -42,
        vadGateInactiveDb: -56,
        vadGateActiveMs: 12,
        vadGateInactiveMs: 18,
        speakerMeanHz: 180,
      },
    });

    await analyzer.loadModel("/headaudio/model-en-mixed.bin");

    analyzer.onvalue = (key, value) => {
      if (window.avatarLipSync && typeof window.avatarLipSync.setVisemeWeight === "function") {
        window.avatarLipSync.setVisemeWeight(key, value);
      }
    };
    analyzer.onstarted = () => {
      state.visemeEnabled = true;
      pushStatus("WASM 嘴型分析已啟動。");
    };
    analyzer.onended = () => {
      state.visemeEnabled = false;
      if (window.avatarLipSync && typeof window.avatarLipSync.clearVisemes === "function") {
        window.avatarLipSync.clearVisemes();
      }
    };
    analyzer.onprocessorerror = (event) => {
      console.error(event);
      pushStatus("嘴型分析處理器發生錯誤。");
    };

    state.playbackDelayNode = new DelayNode(state.playerContext, { delayTime: 0.08 });
    state.playbackDelayNode.connect(state.fallbackAnalyser);
    state.visemeAnalyzer = analyzer;
    startVisemeLoop();
  })();

  await state.visemeReady;
}

function startVisemeLoop() {
  if (state.visemeRAF) return;
  state.visemeLastTick = performance.now();

  const tick = (now) => {
    if (state.visemeAnalyzer) {
      const dt = now - state.visemeLastTick;
      state.visemeAnalyzer.update(Math.max(0, dt));
      state.visemeLastTick = now;
    }
    if (!state.visemeEnabled && state.fallbackAnalyser) {
      const data = new Uint8Array(state.fallbackAnalyser.frequencyBinCount);
      state.fallbackAnalyser.getByteTimeDomainData(data);
      let sum = 0;
      for (let i = 0; i < data.length; i++) {
        const v = (data[i] - 128) / 128.0;
        sum += v * v;
      }
      const rms = Math.sqrt(sum / data.length);
      const val = Math.min(1, rms * 5.0);
      if (window.avatarLipSync && typeof window.avatarLipSync.setMouthOpen === "function") {
        window.avatarLipSync.setMouthOpen(val > 0.05 ? val : 0);
      }
    }
    state.visemeRAF = window.requestAnimationFrame(tick);
  };

  state.visemeRAF = window.requestAnimationFrame(tick);
}

function stopVisemeLoop() {
  if (state.visemeRAF) {
    window.cancelAnimationFrame(state.visemeRAF);
    state.visemeRAF = 0;
  }
}

function updateStatus(text) {
  el.status.textContent = text;
}

function setConnectionState(nextState) {
  state.connectionState = nextState;
  const labels = {
    connected: "已連線",
    connecting: "連線中",
    disconnected: "未連線",
  };
  const statusText = {
    connected: "已連線",
    connecting: "連線中...",
    disconnected: "未連線",
  };
  el.connectionLight.classList.toggle("connected", nextState === "connected");
  el.connectionLabel.textContent = labels[nextState] || labels.disconnected;
  updateStatus(statusText[nextState] || statusText.disconnected);
}

function debug(text) {
  el.debug.textContent = text;
}

function addBubble(type, text) {
  const node = document.createElement("div");
  node.className = `bubble ${type}`;
  node.textContent = text;
  el.feed.appendChild(node);
  el.feed.scrollTop = el.feed.scrollHeight;
  return node;
}

function resetAssistantTurn() {
  state.activeAssistantBubble = null;
}

function beginSpeechTurn() {
  if (state.inputTurnActive) return;
  state.inputTurnActive = true;
  resetAssistantTurn();
  state.outputInterrupted = false;
  state.outputTranscriptText = "";
  setTranscript("output", "", "助理正在準備回覆。");
}

function appendAssistantText(text) {
  const chunk = String(text || "");
  if (!chunk) return null;
  if (!state.activeAssistantBubble || !state.activeAssistantBubble.isConnected) {
    state.activeAssistantBubble = addBubble("assistant", chunk);
    return state.activeAssistantBubble;
  }
  state.activeAssistantBubble.textContent += chunk;
  el.feed.scrollTop = el.feed.scrollHeight;
  return state.activeAssistantBubble;
}

function shouldHideAssistantText(text) {
  const value = String(text || "").trim();
  if (!value) return true;

  if (ASSISTANT_TEXT_FILTER_RULES.whitelistPatterns.some((pattern) => pattern.test(value))) {
    return false;
  }

  const normalized = value.replace(/\s+/g, " ").trim();
  const lines = value.split(/\n+/).map((line) => line.trim()).filter(Boolean);
  const lower = normalized.toLowerCase();

  const headingLike = lines.length >= 2 && ASSISTANT_TEXT_FILTER_RULES.structuralPatterns.headingLike.test(lines[0]);
  const quotedDirective = ASSISTANT_TEXT_FILTER_RULES.blacklistKeywordPatterns.some((pattern) => pattern.test(lower));
  const internalProcessTone = ASSISTANT_TEXT_FILTER_RULES.blacklistTonePatterns.some((pattern) => pattern.test(lower));
  const strongEnglishOnly = ASSISTANT_TEXT_FILTER_RULES.structuralPatterns.strongEnglishOnly.test(value)
    && !ASSISTANT_TEXT_FILTER_RULES.whitelistPatterns.some((pattern) => pattern.test(value));

  const score = [
    headingLike,
    quotedDirective,
    internalProcessTone,
    strongEnglishOnly && lines.length >= 2 && normalized.length > ASSISTANT_TEXT_FILTER_RULES.minStructuredEnglishLength,
  ].filter(Boolean).length;

  return score >= ASSISTANT_TEXT_FILTER_RULES.minBlacklistScore;
}

function pushStatus(text) {
  if (!text) return;
  const node = document.createElement("div");
  node.className = "status-line";
  node.textContent = text;
  el.statusFeed.appendChild(node);
  while (el.statusFeed.children.length > 10) {
    el.statusFeed.removeChild(el.statusFeed.firstChild);
  }
  el.statusFeed.scrollTop = el.statusFeed.scrollHeight;
  refreshStatusFeedVisibility();
}

function setTranscript(kind, text, placeholder) {
  const target = kind === "input" ? el.inputTranscript : el.outputTranscript;
  const value = (text || "").trim();
  target.classList.remove("interrupted");
  if (!value) {
    target.textContent = placeholder;
    target.classList.add("empty");
    refreshLivePaneVisibility();
    return;
  }
  target.textContent = value;
  target.classList.remove("empty");
  refreshLivePaneVisibility();
}

function setLLMPrompt(systemPrompt, userPrompt) {
  const sections = [];
  if ((systemPrompt || "").trim()) {
    sections.push(`[系統]\n${systemPrompt.trim()}`);
  }
  if ((userPrompt || "").trim()) {
    sections.push(`[使用者]\n${userPrompt.trim()}`);
  }

  if (sections.length === 0) {
    el.llmPrompt.textContent = "正在等待語音辨識結果。";
    el.llmPrompt.classList.add("empty");
    refreshLivePaneVisibility();
    return;
  }

  el.llmPrompt.textContent = sections.join("\n\n");
  el.llmPrompt.classList.remove("empty");
  refreshLivePaneVisibility();
}

function markAssistantTranscriptInterrupted() {
  const current = (el.outputTranscript.textContent || "").trim();
  if (!current || el.outputTranscript.classList.contains("empty")) {
    return;
  }
  if (state.outputInterrupted) {
    return;
  }
  el.outputTranscript.textContent = `${current}\n[已中斷]`;
  el.outputTranscript.classList.remove("empty");
  el.outputTranscript.classList.add("interrupted");
  state.outputInterrupted = true;
  refreshLivePaneVisibility();
}

function mergeProgressiveText(current, incoming) {
  const a = (current || "").trim();
  const b = (incoming || "").trim();
  if (!a) return b;
  if (!b) return a;
  if (b.startsWith(a)) return b;
  if (a.startsWith(b)) return a;
  if (a.includes(b)) return a;
  return a + b;
}

function wsUrl() {
  const protocol = location.protocol === "https:" ? "wss:" : "ws:";
  const url = new URL(`${protocol}//${location.host}/ws/live`);
  if (state.sessionId) {
    url.searchParams.set("session_id", state.sessionId);
  }
  return url.toString();
}

async function createSessionIfNeeded() {
  if (state.sessionId) return state.sessionId;

  const response = await fetch("/api/session/create", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({}),
  });
  if (!response.ok) {
    throw new Error(`failed to create session: ${response.status}`);
  }

  const payload = await response.json();
  state.sessionId = payload.sessionId || "";
  renderSessionMeta();
  pushStatus(`已建立工作階段：${state.sessionId}`);
  return state.sessionId;
}

async function loadAvatars() {
  const response = await fetch("/api/avatars");
  if (!response.ok) {
    throw new Error(`failed to load avatars: ${response.status}`);
  }
  const payload = await response.json();
  state.avatars = Array.isArray(payload.avatars) ? payload.avatars : [];
  const preferredAvatarId = document.body.dataset.avatarId || "";
  const hasPreferredAvatar = preferredAvatarId && state.avatars.some((avatar) => avatar.id === preferredAvatarId);
  state.selectedAvatarId = hasPreferredAvatar
    ? preferredAvatarId
    : (payload.defaultAvatarId || state.avatars[0]?.id || "");
  renderAvatarOptions();
  applySelectedAvatar();
  await connect();
}

function renderAvatarOptions() {
  el.avatarSelect.innerHTML = "";
  for (const avatar of state.avatars) {
    const option = document.createElement("option");
    option.value = avatar.id;
    option.textContent = avatar.displayName || avatar.id;
    if (avatar.id === state.selectedAvatarId) {
      option.selected = true;
    }
    el.avatarSelect.appendChild(option);
  }
}

function onAvatarChange() {
  state.selectedAvatarId = el.avatarSelect.value;
  applySelectedAvatar();
  if (window.avatarRuntime && typeof window.avatarRuntime.loadAvatar === "function") {
    window.avatarRuntime.loadAvatar(getSelectedAvatar()?.avatarModel || "/avatar/friday.vrm");
  }
}

function getSelectedAvatar() {
  return state.avatars.find((avatar) => avatar.id === state.selectedAvatarId) || null;
}

function applySelectedAvatar() {
  const avatar = getSelectedAvatar();
  if (!avatar) return;
  if ((avatar.model || "").trim()) {
    el.model.value = avatar.model.trim();
  }
  if ((avatar.voice || "").trim()) {
    el.voice.value = avatar.voice.trim();
  }
  if (typeof avatar.temperature === "number" && Number.isFinite(avatar.temperature)) {
    el.temperature.value = String(avatar.temperature);
  }
  if (typeof avatar.enableInputTranscription === "boolean") {
    el.inputTranscription.checked = avatar.enableInputTranscription;
  }
  if (typeof avatar.enableOutputTranscription === "boolean") {
    el.outputTranscription.checked = avatar.enableOutputTranscription;
  }
  if ((avatar.systemInstruction || "").trim()) {
    el.systemInstruction.value = avatar.systemInstruction.trim();
  }
  const heroName = avatar.displayName || avatar.id || "角色";
  el.heroName.textContent = heroName;
  el.heroSubtitle.textContent = `${heroName} 已透過 Gemini Live 語音代理準備完成。瀏覽器只會連到你的伺服器，由伺服器持有 Gemini Live 連線。`;
  document.title = `${heroName} | Gemini Live 語音代理`;
  renderAvatarSummary(avatar);
  if (window.avatarRuntime && typeof window.avatarRuntime.loadAvatar === "function") {
    window.avatarRuntime.loadAvatar(avatar.avatarModel || "/avatar/friday.vrm");
  }
  pushStatus(`已選擇角色：${avatar.displayName || avatar.id}`);
}

function renderAvatarSummary(avatar) {
  const specialRules = Array.isArray(avatar.specialRules) && avatar.specialRules.length > 0
    ? avatar.specialRules.join(" / ")
    : "無";
  const summary = [
    ["查詢來源", avatar.querySource || "未知"],
    ["端點", avatar.endpoint || "未設定"],
    ["模型", avatar.model || el.model.value || "未設定"],
    ["語音", avatar.voice || el.voice.value || "未設定"],
    ["溫度", typeof avatar.temperature === "number" ? String(avatar.temperature) : (el.temperature.value || "未設定")],
    ["輸入逐字稿", typeof avatar.enableInputTranscription === "boolean" ? (avatar.enableInputTranscription ? "開啟" : "關閉") : (el.inputTranscription.checked ? "開啟" : "關閉")],
    ["輸出逐字稿", typeof avatar.enableOutputTranscription === "boolean" ? (avatar.enableOutputTranscription ? "開啟" : "關閉") : (el.outputTranscription.checked ? "開啟" : "關閉")],
    ["角色模型", avatar.avatarModel || "未設定"],
    ["特殊規則", specialRules],
  ];

  el.avatarSummary.innerHTML = summary.map(([label, value]) => `
    <li class="summary-item">
      <span class="summary-label">${escapeHtml(label)}:</span>
      <span class="summary-value">${escapeHtml(value)}</span>
    </li>
  `).join("");
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll("\"", "&quot;")
    .replaceAll("'", "&#39;");
}

async function connect() {
  if (state.ws && state.ws.readyState <= 1) return;
  await createSessionIfNeeded();

  setConnectionState("connecting");
  state.ws = new WebSocket(wsUrl());
  debug(`正在開啟工作階段 ${state.sessionId || "未知"} 的代理連線`);

  state.ws.onopen = () => {
    setConnectionState("connected");
    debug(`工作階段 ${state.sessionId || "未知"} 的代理連線已就緒`);
    ensureVisemeAnalyzer().catch((error) => {
      console.error(error);
      pushStatus(`嘴型分析初始化失敗：${error.message}`);
    });
    state.ws.send(JSON.stringify({
      type: "setup",
      sessionId: state.sessionId,
      setup: {
        avatarId: state.selectedAvatarId,
        model: el.model.value.trim(),
        systemInstruction: el.systemInstruction.value.trim(),
        voice: el.voice.value,
        temperature: parseFloat(el.temperature.value),
        enableInputTranscription: el.inputTranscription.checked,
        enableOutputTranscription: el.outputTranscription.checked,
      },
    }));
  };

  state.ws.onmessage = async (event) => {
    const payload = JSON.parse(event.data);
    if (payload.type === "ready") {
      if (payload.message?.sessionId) {
        state.sessionId = payload.message.sessionId;
        renderSessionMeta();
      }
      pushStatus(`即時語音工作階段已就緒：${payload.message.model}（${payload.message.avatarId || state.selectedAvatarId}）`);
      return;
    }
    if (payload.type === "status") {
      pushStatus(String(payload.message));
      debug(String(payload.message));
      if (String(payload.message).includes("Previous task canceled") || String(payload.message).includes("replaced the previous task")) {
        markAssistantTranscriptInterrupted();
      }
      return;
    }
    if (payload.type === "llm_prompt") {
      setLLMPrompt(payload.message?.systemPrompt || "", payload.message?.userPrompt || "");
      pushStatus("已顯示送往自訂 LLM 的提示詞。");
      return;
    }
    if (payload.type === "assistant_text") {
      const text = String(payload.message?.text || "").trim();
      if (text && !shouldHideAssistantText(text)) {
        resetAssistantTurn();
        appendAssistantText(text);
      }
      return;
    }
    if (payload.type === "error") {
      pushStatus(`錯誤：${payload.error}`);
      debug(payload.error);
      return;
    }
    if (payload.type === "live") {
      handleLiveMessage(payload.message);
    }
  };

  state.ws.onclose = () => {
    state.ws = null;
    setConnectionState("disconnected");
    debug("連線已關閉");
    stopMic();
  };

  state.ws.onerror = () => {
    setConnectionState("disconnected");
    debug("連線發生錯誤");
  };
}

function disconnect() {
  stopMic();
  if (state.ws) {
    state.ws.close();
    state.ws = null;
  }
  state.nextPlaybackTime = 0;
  state.visemeEnabled = false;
  state.inputTranscriptText = "";
  state.inputTurnActive = false;
  state.outputTranscriptText = "";
  state.outputInterrupted = false;
  resetAssistantTurn();
  if (window.avatarLipSync && typeof window.avatarLipSync.clearVisemes === "function") {
    window.avatarLipSync.clearVisemes();
  }
  setTranscript("input", "", "正在等待語音辨識。");
  setTranscript("output", "", "正在等待助理回覆。");
  setLLMPrompt("", "");
  setConnectionState("disconnected");
  debug(`已中斷工作階段 ${state.sessionId || "未知"}`);
}

function sendText() {
  const text = el.textInput.value.trim();
  if (!text || !state.ws || state.ws.readyState !== WebSocket.OPEN) return;
  addBubble("user", text);
  state.inputTurnActive = false;
  resetAssistantTurn();
  state.outputInterrupted = false;
  state.outputTranscriptText = "";
  setTranscript("output", "", "助理正在準備回覆。");
  state.ws.send(JSON.stringify({ type: "text", text }));
  el.textInput.value = "";
}

function handleLiveMessage(message) {
  const content = message.serverContent;
  if (!content) return;

  if (content.inputTranscription?.text) {
    const text = content.inputTranscription.text.trim();
    if (text) {
      beginSpeechTurn();
      state.inputTranscriptText = mergeProgressiveText(state.inputTranscriptText, text);
    }
    setTranscript("input", state.inputTranscriptText, "正在等待語音辨識。");
    if (content.inputTranscription.finished) {
      pushStatus("語音辨識完成。");
      state.inputTranscriptText = "";
      state.inputTurnActive = false;
    }
  }

  if (content.modelTurn?.parts?.length) {
    for (const part of content.modelTurn.parts) {
      if (part.text && !shouldHideAssistantText(part.text)) {
        appendAssistantText(part.text);
      }
      if (part.inlineData?.data) {
        playAudioChunk(part.inlineData.data);
      }
    }
  }

  if (content.outputTranscription?.text) {
    const text = content.outputTranscription.text.trim();
    if (text) {
      state.outputTranscriptText = mergeProgressiveText(state.outputTranscriptText, text);
    }
    state.outputInterrupted = false;
    setTranscript("output", state.outputTranscriptText, "正在等待助理回覆。");
    if (content.outputTranscription.finished) {
      pushStatus("助理語音播放完成。");
      state.outputTranscriptText = "";
    }
  }

  if (content.interrupted) {
    state.nextPlaybackTime = 0;
    state.inputTurnActive = false;
    markAssistantTranscriptInterrupted();
    pushStatus("助理播放已被新的使用者語音中斷。");
  }
}

async function toggleMic() {
  if (!state.micOn) {
    await startMic();
  } else {
    stopMic();
  }
}

async function startMic() {
  if (!state.ws || state.ws.readyState !== WebSocket.OPEN) {
    pushStatus("請先連線。");
    return;
  }

  state.mediaStream = await navigator.mediaDevices.getUserMedia({
    audio: {
      channelCount: 1,
      sampleRate: 16000,
      echoCancellation: true,
      noiseSuppression: true,
      autoGainControl: true,
    },
  });

  state.audioContext = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 16000 });
  const source = state.audioContext.createMediaStreamSource(state.mediaStream);
  const processor = state.audioContext.createScriptProcessor(4096, 1, 1);

  processor.onaudioprocess = (event) => {
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) return;
    const input = event.inputBuffer.getChannelData(0);
    const pcm16 = floatTo16BitPCM(input);
    const base64 = arrayBufferToBase64(pcm16.buffer);
    state.ws.send(JSON.stringify({ type: "audio_chunk", audio: base64 }));
  };

  source.connect(processor);
  processor.connect(state.audioContext.destination);

  state.captureSource = source;
  state.captureNode = processor;
  state.micOn = true;
  state.inputTurnActive = false;
  state.outputInterrupted = false;
  state.outputTranscriptText = "";
  refreshLivePaneVisibility();
  setTranscript("output", "", "助理正在準備回覆。");
  el.micBtn.textContent = "麥克風開啟";
  el.micBtn.classList.remove("stop");
  pushStatus("麥克風串流已啟動。");
}

function stopMic() {
  if (state.captureSource) {
    state.captureSource.disconnect();
    state.captureSource = null;
  }
  if (state.captureNode) {
    state.captureNode.disconnect();
    state.captureNode = null;
  }
  if (state.mediaStream) {
    state.mediaStream.getTracks().forEach((track) => track.stop());
    state.mediaStream = null;
  }
  if (state.audioContext) {
    state.audioContext.close();
    state.audioContext = null;
  }
  if (state.micOn && state.ws && state.ws.readyState === WebSocket.OPEN) {
    state.ws.send(JSON.stringify({ type: "audio_end" }));
  }
  state.micOn = false;
  refreshLivePaneVisibility();
  state.inputTranscriptText = "";
  state.inputTurnActive = false;
  setTranscript("input", "", "正在等待語音辨識。");
  el.micBtn.textContent = "麥克風關閉";
  el.micBtn.classList.add("stop");
}

async function playAudioChunk(base64Data) {
  await ensureVisemeAnalyzer();
  if (state.playerContext.state === "suspended") {
    await state.playerContext.resume();
  }

  const bytes = base64ToUint8Array(base64Data);
  const pcm = new Int16Array(bytes.buffer);
  if (!state.visemeEnabled) {
    // Relying on real-time fallback analyser in the viseme loop.
  }
  const audioBuffer = state.playerContext.createBuffer(1, pcm.length, 24000);
  const channel = audioBuffer.getChannelData(0);
  for (let i = 0; i < pcm.length; i += 1) {
    channel[i] = pcm[i] / 32768;
  }

  const source = state.playerContext.createBufferSource();
  source.buffer = audioBuffer;
  if (state.playbackDelayNode) {
    source.connect(state.playbackDelayNode);
  } else {
    source.connect(state.fallbackAnalyser || state.playerContext.destination);
  }
  if (state.visemeAnalyzer) {
    source.connect(state.visemeAnalyzer);
  }
  const now = state.playerContext.currentTime;
  if (state.nextPlaybackTime < now) {
    state.nextPlaybackTime = now + 0.02;
  }
  source.start(state.nextPlaybackTime);
  state.nextPlaybackTime += audioBuffer.duration;
}

function floatTo16BitPCM(float32Array) {
  const output = new Int16Array(float32Array.length);
  for (let i = 0; i < float32Array.length; i += 1) {
    const sample = Math.max(-1, Math.min(1, float32Array[i]));
    output[i] = sample < 0 ? sample * 0x8000 : sample * 0x7fff;
  }
  return output;
}

function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function base64ToUint8Array(base64) {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function estimateMouthOpenFromPCM(pcm) {
  if (!pcm || pcm.length === 0) return 0;
  let sumSquares = 0;
  let peak = 0;
  for (let i = 0; i < pcm.length; i += 1) {
    const normalized = Math.abs(pcm[i] / 32768);
    sumSquares += normalized * normalized;
    if (normalized > peak) peak = normalized;
  }
  const rms = Math.sqrt(sumSquares / pcm.length);
  const value = Math.min(1, rms * 4.5 + peak * 0.35);
  return value < 0.03 ? 0 : value;
}

function estimateVowelPoseFromPCM(pcm) {
  const open = estimateMouthOpenFromPCM(pcm);
  if (open <= 0) {
    return { aa: 0, ih: 0, ou: 0, ee: 0, oh: 0 };
  }

  let zeroCrossings = 0;
  let previous = pcm[0] || 0;
  let brightEnergy = 0;
  const step = Math.max(1, Math.floor(pcm.length / 128));

  for (let i = step; i < pcm.length; i += step) {
    const current = pcm[i];
    if ((previous >= 0 && current < 0) || (previous < 0 && current >= 0)) {
      zeroCrossings += 1;
    }
    brightEnergy += Math.abs(current - previous);
    previous = current;
  }

  const zcr = Math.min(1, zeroCrossings / 40);
  const brightness = Math.min(1, brightEnergy / (pcm.length * 1800));
  const rounded = 1 - Math.min(1, brightness * 0.9 + zcr * 0.55);

  const aa = open * (0.55 + rounded * 0.45);
  const oh = open * (0.35 + rounded * 0.4);
  const ou = open * (0.25 + rounded * 0.5);
  const ee = open * Math.max(0, brightness * 0.95 - rounded * 0.15);
  const ih = open * Math.max(0, brightness * 0.7);

  return normalizePose({ aa, ih, ou, ee, oh });
}

function normalizePose(pose) {
  const maxValue = Math.max(pose.aa, pose.ih, pose.ou, pose.ee, pose.oh, 0);
  if (maxValue <= 0) {
    return { aa: 0, ih: 0, ou: 0, ee: 0, oh: 0 };
  }
  return {
    aa: Math.min(1, pose.aa),
    ih: Math.min(1, pose.ih),
    ou: Math.min(1, pose.ou),
    ee: Math.min(1, pose.ee),
    oh: Math.min(1, pose.oh),
  };
}

function driveAvatarMouth(value) {
  if (state.visemeEnabled) {
    return;
  }
  if (window.avatarLipSync && typeof window.avatarLipSync.setMouthOpen === "function") {
    window.avatarLipSync.setMouthOpen(value);
  }
  if (state.mouthCloseTimer) {
    clearTimeout(state.mouthCloseTimer);
  }
  state.mouthCloseTimer = setTimeout(() => {
    if (window.avatarLipSync && typeof window.avatarLipSync.setMouthOpen === "function") {
      window.avatarLipSync.setMouthOpen(0);
    }
    if (window.avatarLipSync && typeof window.avatarLipSync.setMouthPose === "function") {
      window.avatarLipSync.setMouthPose({ aa: 0, ih: 0, ou: 0, ee: 0, oh: 0 });
    }
  }, 90);
}

window.addEventListener("DOMContentLoaded", init);
